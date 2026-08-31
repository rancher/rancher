package system

//go:generate go tool -modfile ../../../gotools/mockery/go.mod mockery --name HelmClient --output ./mocks --case underscore
//go:generate go tool -modfile ../../../gotools/mockery/go.mod mockery --name OperationClient --output ./mocks --case underscore
//go:generate go tool -modfile ../../../gotools/mockery/go.mod mockery --name ContentClient --output ./mocks --case underscore
//go:generate go tool -modfile ../../../gotools/mockery/go.mod mockery --name PodClient --output ./mocks --case underscore --srcpkg github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1
//go:generate go tool -modfile ../../../gotools/mockery/go.mod mockery --name SettingController --output ./mocks --case underscore --srcpkg github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3
//go:generate go tool -modfile ../../../gotools/mockery/go.mod mockery --name ClusterRepoController --output ./mocks --case underscore --srcpkg github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v4/pkg/action"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

var (
	installUser = &user.DefaultInfo{
		Name: "helm-installer",
		UID:  "helm-installer",
		Groups: []string{
			"system:masters",
		},
	}
)

type desiredKey struct {
	namespace            string
	chartName            string
	releaseName          string
	minVersion           string
	exactVersion         string
	installImageOverride string
}

type desired struct {
	key           desiredKey
	values        map[string]interface{}
	takeOwnership bool
}

type HelmClient interface {
	// ListReleases lists all releases in the given namespace that matches the given name and stateMask
	ListReleases(namespace, name string, stateMask action.ListStates) ([]*release.Release, error)
}

type OperationClient interface {
	// Upgrade gets the upgrade commands using the given namespace, name and options and gets the user using the isApp flag as false.
	// Returns a catalog.Operation that represents the helm operation to be created
	Upgrade(ctx context.Context, user user.Info, namespace, name string, options io.Reader, imageOverride string) (*catalog.Operation, error)
	// Uninstall gets the uninstallation commands using the given namespace, name and options and gets the user information using the isApp flag as true.
	// Returns a catalog.Operation that represents the helm operation to be created
	Uninstall(ctx context.Context, user user.Info, namespace, name string, options io.Reader, imageOverride string) (*catalog.Operation, error)
	// AddCpTaintsToTolerations gets the list of control plane nodes and adds their taints to the given tolerations.
	AddCpTaintsToTolerations(tolerations []v1.Toleration) ([]v1.Toleration, error)
}

type ContentClient interface {
	// Index receives a repository's name and namespace and returns its index file.
	Index(namespace, name, targetK8sVersion string, skipFilter bool) (*repo.IndexFile, error)
}

type Manager struct {
	ctx                   context.Context
	operation             OperationClient
	content               ContentClient
	pods                  corecontrollers.PodClient
	desiredCharts         map[desiredKey]map[string]interface{}
	sync                  chan desired
	refreshIntervalChange chan struct{}
	settings              mgmtcontrollers.SettingController
	trigger               chan struct{}
	clusterRepos          catalogcontrollers.ClusterRepoController
	helmClient            HelmClient
}

func NewManager(ctx context.Context,
	contentManager ContentClient,
	ops OperationClient,
	pods corecontrollers.PodClient,
	settings mgmtcontrollers.SettingController,
	clusterRepos catalogcontrollers.ClusterRepoController,
	helmClient HelmClient) (*Manager, error) {

	m := &Manager{
		ctx:                   ctx,
		operation:             ops,
		content:               contentManager,
		pods:                  pods,
		sync:                  make(chan desired, 10),
		desiredCharts:         map[desiredKey]map[string]interface{}{},
		refreshIntervalChange: make(chan struct{}, 1),
		settings:              settings,
		trigger:               make(chan struct{}, 1),
		clusterRepos:          clusterRepos,
		helmClient:            helmClient,
	}

	return m, nil
}

func (m *Manager) Start(ctx context.Context) {
	m.ctx = ctx
	go m.runSync()

	m.settings.OnChange(ctx, "system-feature-chart-refresh", m.onSetting)
	m.clusterRepos.OnChange(ctx, "catalog-refresh-trigger", m.onTrigger)
}

func (m *Manager) onSetting(key string, obj *v3.Setting) (*v3.Setting, error) {
	if key != settings.SystemFeatureChartRefreshSeconds.Name {
		return obj, nil
	}

	m.refreshIntervalChange <- struct{}{}
	return obj, nil
}

func (m *Manager) onTrigger(_ string, obj *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	// We only want to trigger on "rancher-charts" in order to ensure that required charts, such as
	// Fleet, are up-to-date upon Rancher startup or upgrade.
	if obj == nil || obj.DeletionTimestamp != nil || obj.Name != "rancher-charts" {
		return obj, nil
	}

	select {
	case m.trigger <- struct{}{}:
	default:
	}
	return obj, nil
}

// runSync owns m.desiredCharts. Installs themselves run on worker goroutines, bounded by
// maxConcurrentInstalls, and report back on the results channel so that the map is still only
// ever touched from this goroutine.
//
// Installs used to run inline here, one at a time. Because install waits for the helm operation
// pod to finish — and the helm action sets Wait: true, so that means waiting for the installed
// workload to become Ready — a single slow chart delayed every other chart, the refresh ticker,
// and any pending Ensure. On a new downstream cluster that made two independent charts take the
// sum of their install times instead of the longer of the two.
func (m *Manager) runSync() {
	t := time.NewTicker(getIntervalOrDefault(settings.SystemFeatureChartRefreshSeconds.Get()))
	defer func() { t.Stop() }()

	var (
		results  = make(chan installResult, maxConcurrentInstalls)
		sem      = make(chan struct{}, maxConcurrentInstalls)
		inFlight = map[desiredKey]struct{}{}
	)

	// dispatch starts an install unless one is already running for this key. It must only be
	// called from this goroutine.
	dispatch := func(d desired) {
		if _, busy := inFlight[d.key]; busy {
			return
		}
		inFlight[d.key] = struct{}{}

		go func() {
			select {
			case sem <- struct{}{}:
			case <-m.ctx.Done():
				return
			}
			defer func() { <-sem }()

			err := m.installOne(d.key, d.values, d.takeOwnership)
			select {
			case results <- installResult{key: d.key, values: d.values, err: err}:
			case <-m.ctx.Done():
			}
		}()
	}

	// dispatchDesired re-drives everything already known to be desired, e.g. on the refresh
	// ticker. Reading the map here keeps it on this goroutine.
	dispatchDesired := func() {
		for key, values := range m.desiredCharts {
			dispatch(desired{key: key, values: values, takeOwnership: true})
		}
	}

	for {
		select {
		case <-m.refreshIntervalChange:
			t.Stop()
			t = time.NewTicker(getIntervalOrDefault(settings.SystemFeatureChartRefreshSeconds.Get()))
		case <-m.ctx.Done():
			return
		case <-m.trigger:
			dispatchDesired()
		case <-t.C:
			dispatchDesired()
		case d := <-m.sync:
			// newly requested or changed
			if v, exists := m.desiredCharts[d.key]; exists && equality.Semantic.DeepEqual(v, d.values) {
				continue
			}
			dispatch(d)
		case r := <-results:
			delete(inFlight, r.key)
			if r.err == nil {
				m.desiredCharts[r.key] = r.values
			}
		}
	}
}

// getIntervalOrDefault Converts the input to a time.Duration or returns a default value
func getIntervalOrDefault(interval string) time.Duration {
	i, err := strconv.Atoi(interval)
	if err != nil {
		return 21600 * time.Second
	}
	return time.Duration(i) * time.Second
}

// installCharts installs charts with takeOwnership.
//
// A chart that is not in the repo index yet is retried by re-enqueuing the ClusterRepo
// rather than by sleeping here. installCharts runs on the single runSync goroutine, so
// blocking on one chart also stalls every other chart, the refresh ticker, and any pending
// Ensure calls. On a fresh cluster, where the index is still being built, that is how one
// not-yet-indexed chart delays all of them indefinitely.
func (m *Manager) installCharts(charts map[desiredKey]map[string]interface{}, takeOwnership bool) error {
	var errs []error

	for key, values := range charts {
		if err := m.installOne(key, values, takeOwnership); err != nil {
			errs = append(errs, err)
		}
	}
	return merr.NewErrors(errs...)
}

// isIndexNotReady reports whether err means the rancher-charts index is not available yet,
// which is the normal state of a freshly started cluster agent and is retriable.
//
// Deliberately does not cover "the index exists but lacks the pinned version": the index
// ConfigMap is written complete, so that is a version mismatch rather than a transient state,
// and retrying it every few seconds forever would be worse than surfacing it.
func isIndexNotReady(err error) bool {
	return apierrors.IsNotFound(err) || errors.Is(err, repo.ErrNoChartName)
}

// installOne installs a single chart and classifies the outcome. A not-yet-built index
// schedules a retry; any error is returned so the caller does not record the chart as
// installed.
func (m *Manager) installOne(key desiredKey, values map[string]interface{}, takeOwnership bool) error {
	err := m.install(key.namespace, key.chartName, key.releaseName, key.minVersion, key.exactVersion, values, takeOwnership, key.installImageOverride)
	switch {
	case err == nil:
		return nil
	case isIndexNotReady(err):
		logrus.Infof("System chart %s is not in the %s index yet, retrying in %s: %v",
			key.chartName, systemChartsRepoName, chartNotInIndexRetryDelay, err)
		m.retryInstallLater()
	default:
		logrus.Errorf("Failed to install system chart %s (release name: %s): %v", key.chartName, key.releaseName, err)
	}
	return err
}

// retryInstallLater re-drives the ClusterRepo handler, which re-evaluates the chart list and
// pushes anything still desired back onto the sync channel.
func (m *Manager) retryInstallLater() {
	if m.clusterRepos == nil {
		return
	}
	m.clusterRepos.EnqueueAfter(systemChartsRepoName, chartNotInIndexRetryDelay)
}

func (m *Manager) Uninstall(namespace, name string) error {
	if ok, err := m.hasStatus(namespace, name, action.ListDeployed|action.ListFailed); err != nil {
		return err
	} else if !ok {
		return nil
	}

	uninstall, err := json.Marshal(types.ChartUninstallAction{
		Timeout: &metav1.Duration{Duration: 5 * time.Minute},
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	op, err := m.operation.Uninstall(m.ctx, installUser, namespace, name, bytes.NewBuffer(uninstall), "")
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return m.waitPodDone(op)
}

// Ensure requests that a chart be installed. Requests are queued in call order.
//
// This deliberately sends on the channel directly instead of from a goroutine per call. Callers
// pass charts in a meaningful order — getChartsToInstall lists rancher-webhook before
// system-upgrade-controller, and the webhook should go first — and a goroutine per call
// randomised that. The channel is buffered and runSync no longer blocks on installs, so sending
// here does not stall the calling controller.
func (m *Manager) Ensure(namespace, chartName, releaseName, minVersion, exactVersion string, values map[string]interface{}, takeOwnership bool, installImageOverride string) error {
	d := desired{
		key: desiredKey{
			namespace:            namespace,
			chartName:            chartName,
			releaseName:          releaseName,
			minVersion:           minVersion,
			exactVersion:         exactVersion,
			installImageOverride: installImageOverride,
		},
		values:        values,
		takeOwnership: takeOwnership,
	}

	if m.ctx == nil {
		m.sync <- d
		return nil
	}

	select {
	case m.sync <- d:
	case <-m.ctx.Done():
	}
	return nil
}

func (m *Manager) Remove(namespace, releaseName string) {
	for k := range m.desiredCharts {
		if k.namespace == namespace && k.releaseName == releaseName {
			delete(m.desiredCharts, k)
		}
	}
}

// install tries to install a new version of a chart.
// If the exact version is provided, it will try to install it regardless of whether minVersion is provided.
// If minVersion is provided on its own, it will try to install it only if the current version is earlier than
// minVersion.
// A failure to find a chart for a provided version leads to an error being thrown, without any change in state.
// If no version is provided, it will try to install the latest version available.
// If a release with the version to be installed is already installed, or is pending install, upgrade or rollback, this
// does nothing.
func (m *Manager) install(namespace, chartName, releaseName, minVersion, exactVersion string, values map[string]interface{}, takeOwnership bool, installImageOverride string) error {
	index, err := m.content.Index("", "rancher-charts", "", true)
	if err != nil {
		return err
	}

	if releaseName == "" {
		releaseName = chartName
	}

	const latestVersionMatcher = ">=0-a" // latest - special syntax to match everything including pre-release builds

	v := latestVersionMatcher
	var isExact bool

	if exactVersion != "" {
		v = exactVersion
		isExact = true
	} else if minVersion != "" {
		v = minVersion
		// not enforcing exact version install, to keep any possibly newer, already installed version.
		// This prevents automated downgrades of updates done manually through the web UI, eg. for Fleet.
	}

	// This method from the Helm fork doesn't return an error when given a non-existent version, unfortunately.
	// It instead returns the latest version in the index.
	chart, err := index.Get(chartName, v)
	if err != nil {
		// Returned as-is rather than flattened through errors.New(err.Error()): index.Get
		// returns the bare repo.ErrNoChartName sentinel, and installOne classifies on it with
		// errors.Is, which a flattened copy would never match.
		return err
	}
	// Because of the behavior of `index.Get`, we need this check.
	if v != latestVersionMatcher && chart.Version != v {
		return fmt.Errorf("specified version %s doesn't exist in the index", v)
	}

	// If the chart version is already installed, we do nothing
	installed, desiredVersion, desiredValue, err := m.isInstalled(namespace, releaseName, minVersion, chart.Version, isExact, values)
	if err != nil {
		return err
	} else if installed {
		return nil
	}

	// If the release is pending install, upgrade or rollback, we do nothing.
	// If it's not, we proceed to create an operation
	if ok, err := m.hasStatus(namespace, releaseName, action.ListPendingInstall|action.ListPendingUpgrade|action.ListPendingRollback); err != nil {
		return err
	} else if ok {
		return nil
	}

	if desiredValue == nil {
		desiredValue = map[string]interface{}{}
	}
	// if tolerations are already present we don't change them
	if v, ok := desiredValue["tolerations"]; !ok || v == nil {
		var tolerations []v1.Toleration
		tolerations, err = m.operation.AddCpTaintsToTolerations(tolerations)
		if err != nil {
			logrus.Warnf("failed to add tolerations for control plane taints: %v", err)
		} else if len(tolerations) > 0 {
			desiredValue["tolerations"] = tolerations
		}
	}

	t := operationTimeout()
	upgrade, err := json.Marshal(types.ChartUpgradeAction{
		Timeout:                &metav1.Duration{Duration: t},
		Wait:                   true,
		Install:                true,
		MaxHistory:             5,
		Namespace:              namespace,
		TakeOwnership:          takeOwnership,
		AutomaticCPTolerations: true,
		Charts: []types.ChartUpgrade{
			{
				ChartName:   chartName,
				Version:     desiredVersion,
				ReleaseName: releaseName,
				Values:      desiredValue,
				ResetValues: true,
			},
		},
	})
	if err != nil {
		return err
	}

	op, err := m.operation.Upgrade(m.ctx, installUser, "", "rancher-charts", bytes.NewBuffer(upgrade), installImageOverride)
	if err != nil {
		return err
	}

	return m.waitPodDone(op)
}

const (
	// systemChartsRepoName is the ClusterRepo that system charts are installed from.
	systemChartsRepoName = "rancher-charts"

	// chartNotInIndexRetryDelay is how long to wait before re-driving the ClusterRepo
	// handler when a system chart is not in the repo index yet.
	chartNotInIndexRetryDelay = 5 * time.Second

	// maxConcurrentInstalls bounds how many chart installs run at once. Installs are
	// independent helm releases so they do not contend, but each one runs a helm operation pod
	// in the target cluster, so this is kept small.
	maxConcurrentInstalls = 4
)

// installResult carries an install's outcome back to runSync, which owns desiredCharts.
type installResult struct {
	key    desiredKey
	values map[string]interface{}
	err    error
}

// operationTimeout returns how long a helm operation pod is allowed to run, which is also
// how long waitPodDone waits for it.
func operationTimeout() time.Duration {
	t, err := time.ParseDuration(settings.SystemManagedChartsOperationTimeout.Get())
	if err != nil {
		return 5 * time.Minute
	}
	return t
}

// waitPodDone receives an operation, gets its pod and checks if it's done, returning nil if
// it is. Otherwise it watches the pod until it completes or the helm operation's own timeout
// elapses.
//
// The watch is re-established when the API server closes it early rather than being treated
// as a failure. A closed watch says nothing about the pod: on a fresh cluster the first
// install routinely outlives a single watch (image pulls plus Wait:true on the helm action),
// and reporting that as "pod failed, watch closed" makes installCharts record an error, drop
// the chart from desiredCharts, and not retry it until the next ClusterRepo event — up to an
// hour later.
func (m *Manager) waitPodDone(op *catalog.Operation) error {
	pod, err := m.pods.Get(op.Status.PodNamespace, op.Status.PodName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if ok, err := podDone(op.Status.Chart, pod); err != nil {
		return err
	} else if ok {
		return nil
	}

	timeout := operationTimeout()
	deadline := time.Now().Add(timeout)
	resourceVersion := pod.ResourceVersion

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out after %s waiting for pod %s/%s to complete", timeout, pod.Namespace, pod.Name)
		}

		done, lastResourceVersion, err := m.watchPodOnce(op, pod, resourceVersion, remaining)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if lastResourceVersion != "" {
			resourceVersion = lastResourceVersion
		}
	}
}

// watchPodOnce watches a single pod until it completes, the watch closes, or timeout elapses.
// It reports whether the pod completed, plus the last resourceVersion it observed so the
// caller can resume without replaying events it has already seen.
func (m *Manager) watchPodOnce(op *catalog.Operation, pod *v1.Pod, resourceVersion string, timeout time.Duration) (bool, string, error) {
	sec := int64(timeout.Seconds()) + 1
	resp, err := m.pods.Watch(op.Status.PodNamespace, metav1.ListOptions{
		FieldSelector:   "metadata.name=" + pod.Name,
		ResourceVersion: resourceVersion,
		TimeoutSeconds:  &sec,
	})
	if err != nil {
		return false, resourceVersion, err
	}
	defer func() {
		go func() {
			//nolint:revive
			for range resp.ResultChan() {
				// Intentionally drain the channel.
			}
		}()
		resp.Stop()
	}()

	for event := range resp.ResultChan() {
		newPod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}
		resourceVersion = newPod.ResourceVersion
		if ok, err := podDone(op.Status.Chart, newPod); err != nil {
			return false, resourceVersion, err
		} else if ok {
			return true, resourceVersion, nil
		}
	}

	return false, resourceVersion, nil
}

// podDone receives a chart name and a pod. It will check all containers in that pod and
// get one named helm to check if it terminated and if it did so successfully.
// If there's no helm container or if the container didn't terminate, it returns false.
func podDone(chart string, newPod *v1.Pod) (bool, error) {
	for _, container := range newPod.Status.ContainerStatuses {
		if container.Name != "helm" {
			continue
		}
		if container.State.Terminated != nil {
			if container.State.Terminated.ExitCode == 0 {
				return true, nil
			}
			return false, fmt.Errorf("failed to install %s, pod %s/%s exited %d", chart,
				newPod.Namespace, newPod.Name, container.State.Terminated.ExitCode)
		}
	}
	return false, nil
}

// isInstalled gets all releases for a particular namespace and name that has the status action.ListDeployed.
// It calls the desiredVersionAndValues function with it to return if the chart is installed, the desired version and the desired values for it.
func (m *Manager) isInstalled(namespace, name, minVersion, desiredVersion string, isExact bool, desiredValue map[string]interface{}) (bool, string, map[string]interface{}, error) {
	releases, err := m.helmClient.ListReleases(namespace, name, action.ListDeployed)
	if err != nil {
		return false, "", nil, err
	}

	return desiredVersionAndValues(releases, minVersion, desiredVersion, isExact, desiredValue)
}

// desiredVersionAndValues returns whether the release is installed. If not, it returns the desired version and Helm values.
// Callers must provide the desired version. If isExact is true, then the resulting value is the desiredVersion, which
// may result in a forced upgrade or downgrade. Otherwise, the desiredVersion signifies the latest version, which may
// or may not be installed, depending on the value of the min version.
func desiredVersionAndValues(releases []*release.Release, minVersion, desiredVersion string, isExact bool, desiredValues map[string]any) (bool, string, map[string]interface{}, error) {
	for _, r := range releases {
		if r.Info.Status != releasecommon.StatusDeployed {
			continue
		}
		if desiredValues == nil {
			desiredValues = map[string]interface{}{}
		}
		releaseConfig := r.Config
		if releaseConfig == nil {
			releaseConfig = map[string]interface{}{}
		}

		desiredValuesJSON, err := json.Marshal(desiredValues)
		if err != nil {
			return false, "", nil, err
		}

		actualValueJSON, err := json.Marshal(releaseConfig)
		if err != nil {
			return false, "", nil, err
		}

		patchedJSON, err := jsonpatch.MergePatch(actualValueJSON, desiredValuesJSON)
		if err != nil {
			return false, "", nil, err
		}

		desiredValues = map[string]interface{}{}
		if err := json.Unmarshal(patchedJSON, &desiredValues); err != nil {
			return false, "", nil, err
		}

		current, err := semver.NewVersion(r.Chart.Metadata.Version)
		if err != nil {
			return false, "", nil, err
		}

		desired, err := semver.NewVersion(desiredVersion)
		if err != nil {
			return false, "", nil, err
		}

		if isExact {
			if !isVersionAndMetadataEqual(current, desired) {
				return false, desired.String(), desiredValues, nil
			}
		}

		if minVersion != "" {
			min, err := semver.NewVersion(minVersion)
			if err != nil {
				return false, "", nil, err
			}
			if desired.LessThan(min) {
				logrus.Errorf("available chart version (%s) for %s is less than the min version (%s) ", desired, r.Chart.Name(), min)
				return false, "", nil, repo.ErrNoChartName
			}
			if min.LessThan(current) || min.Equal(current) {
				// If the current deployed version is greater or equal than the min version but configuration has changed, return false and upgrade with the current version
				if !bytes.Equal(patchedJSON, actualValueJSON) {
					return false, r.Chart.Metadata.Version, desiredValues, nil
				}
				logrus.Debugf("Skipping installing/upgrading desired version %s for release %s, since current version %s is greater or equal to minimal required version %s", desired.String(), r.Name, current.String(), minVersion)
				return true, "", nil, nil
			}
		}

		if (desired.LessThan(current) || desired.Equal(current)) && bytes.Equal(patchedJSON, actualValueJSON) {
			return true, "", nil, nil
		}
	}
	return false, desiredVersion, desiredValues, nil
}

// isVersionAndMetadataEqual is like [semver.Version.Equal] but it also checks whether
// the metadata is the same.
//
// This makes it so that 1.2.3+up4.5.6-rc.1 is not the same as 1.2.3+up4.5.6-rc.2
// because semver ignores anything after the + sign.
//
// Some background on why [semver.Version.Equal] behaves this way: https://github.com/semver/semver/issues/136
func isVersionAndMetadataEqual(v1, v2 *semver.Version) bool {
	return v1.Equal(v2) && v1.Metadata() == v2.Metadata()
}

// hasStatus gets all releases in the given namespace that matches the given name and stateMask and
// returns true if there's any release that matches those conditions.
func (m *Manager) hasStatus(namespace, name string, stateMask action.ListStates) (bool, error) {
	releases, err := m.helmClient.ListReleases(namespace, name, stateMask)
	if err != nil {
		return false, err
	}

	return len(releases) != 0, nil
}
