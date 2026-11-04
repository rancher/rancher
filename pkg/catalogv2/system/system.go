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
	"maps"
	"strconv"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v4/pkg/action"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/repo/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/workqueue"
)

const (
	defaultInterval = 21600 * time.Second
	defaultTimeout  = 5 * time.Minute

	// defaultWorkers is the number of charts that can be installed concurrently from the default queue.
	defaultWorkers = 3
	// priorityWorkers is the number of charts that can be installed concurrently from the priority queue.
	priorityWorkers = 1

	retryBaseDelay        = 5 * time.Second
	retryMaxDelay         = 5 * time.Minute
	priorityRetryMaxDelay = 30 * time.Second
)

var (
	installUser = &user.DefaultInfo{
		Name: "helm-installer",
		UID:  "helm-installer",
		Groups: []string{
			"system:masters",
		},
	}
	errWatch = errors.New("watch channel closed")

	// priorityReleases are installed from a dedicated queue so that they never wait behind other charts.
	// remotedialer-proxy is required for the kube-apiserver to reach the imperative API, which Rancher must be
	// contacted on shortly after startup or it will exit.
	priorityReleases = map[releaseKey]struct{}{
		{namespace: namespace.System, releaseName: "remotedialer-proxy"}: {},
	}
)

// releaseKey identifies a single helm release. Installs are queued by releaseKey so that a release is never
// operated on by more than one worker at a time.
type releaseKey struct {
	namespace   string
	releaseName string
}

type desiredKey struct {
	namespace            string
	chartName            string
	releaseName          string
	minVersion           string
	exactVersion         string
	installImageOverride string
}

// desiredValues has exported members for reflect.DeepEqual
type desiredValues struct {
	Values        map[string]any
	TakeOwnership bool
}

type desired struct {
	key    desiredKey
	values desiredValues
	// refresh is set once the chart is queued by enqueueAll, and cleared when Ensure replaces the chart with new
	// values. See processNextItem.
	refresh bool
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
	desiredMu             sync.RWMutex
	desiredCharts         map[releaseKey]desired
	queue                 workqueue.TypedRateLimitingInterface[releaseKey]
	priorityQueue         workqueue.TypedRateLimitingInterface[releaseKey]
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
		desiredCharts:         map[releaseKey]desired{},
		queue:                 newQueue("system-charts", retryMaxDelay),
		priorityQueue:         newQueue("system-charts-priority", priorityRetryMaxDelay),
		refreshIntervalChange: make(chan struct{}, 1),
		settings:              settings,
		trigger:               make(chan struct{}, 1),
		clusterRepos:          clusterRepos,
		helmClient:            helmClient,
	}

	return m, nil
}

func newQueue(name string, maxDelay time.Duration) workqueue.TypedRateLimitingInterface[releaseKey] {
	return workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.NewTypedItemExponentialFailureRateLimiter[releaseKey](retryBaseDelay, maxDelay),
		workqueue.TypedRateLimitingQueueConfig[releaseKey]{Name: name},
	)
}

func (m *Manager) Start(ctx context.Context) {
	m.ctx = ctx
	for range priorityWorkers {
		go m.runWorker(m.priorityQueue)
	}
	for range getWorkersOrDefault() {
		go m.runWorker(m.queue)
	}
	go func() {
		<-ctx.Done()
		m.priorityQueue.ShutDown()
		m.queue.ShutDown()
	}()
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

func (m *Manager) runSync() {
	t := time.NewTicker(getIntervalOrDefault(settings.SystemFeatureChartRefreshSeconds.Get()))
	defer t.Stop()

	for {
		select {
		case <-m.refreshIntervalChange:
			t.Reset(getIntervalOrDefault(settings.SystemFeatureChartRefreshSeconds.Get()))
		case <-m.ctx.Done():
			return
		case <-m.trigger:
			m.enqueueAll()
		case <-t.C:
			m.enqueueAll()
		}
	}
}

// queueFor returns the queue a release is installed from.
func (m *Manager) queueFor(key releaseKey) workqueue.TypedRateLimitingInterface[releaseKey] {
	if _, ok := priorityReleases[key]; ok {
		return m.priorityQueue
	}
	return m.queue
}

// enqueueAll queues every desired chart to be (re)installed, with ownership taken. See processNextItem.
func (m *Manager) enqueueAll() {
	m.desiredMu.Lock()
	defer m.desiredMu.Unlock()

	for key, chart := range m.desiredCharts {
		chart.refresh = true
		m.desiredCharts[key] = chart
		m.queueFor(key).Add(key)
	}
}

func (m *Manager) runWorker(queue workqueue.TypedRateLimitingInterface[releaseKey]) {
	for m.processNextItem(queue) {
	}
}

// processNextItem makes a single install attempt for the next release in the queue. On failure the release is
// requeued with backoff and the worker moves on, so a failing or slow chart does not block other charts.
func (m *Manager) processNextItem(queue workqueue.TypedRateLimitingInterface[releaseKey]) bool {
	key, shutdown := queue.Get()
	if shutdown {
		return false
	}
	defer queue.Done(key)

	m.desiredMu.RLock()
	// ensure chart hasn't been removed in the meantime
	chart, ok := m.desiredCharts[key]
	m.desiredMu.RUnlock()
	if !ok {
		queue.Forget(key)
		return true
	}

	// The initial install of a new or changed chart uses the takeOwnership requested through Ensure, but refreshes
	// always take ownership. Once installed, fields of a chart's resources can be managed by other field managers;
	// e.g. the caBundle of the rancher-webhook's WebhookConfigurations is managed by the cluster agent. Without
	// taking ownership, which forces server-side apply conflicts, upgrading the release would fail on those fields.
	values := chart.values
	if chart.refresh {
		values.TakeOwnership = true
	}

	if err := m.installChart(chart.key, values); err != nil {
		queue.AddRateLimited(key)
		return true
	}

	queue.Forget(key)
	return true
}

// getIntervalOrDefault Converts the input to a time.Duration or returns a default value
func getIntervalOrDefault(interval string) time.Duration {
	i, err := strconv.Atoi(interval)
	if err != nil {
		return defaultInterval
	}
	return time.Duration(i) * time.Second
}

func (m *Manager) installChart(key desiredKey, values desiredValues) error {
	err := m.install(key.namespace, key.chartName, key.releaseName, key.minVersion, key.exactVersion, values.Values, values.TakeOwnership, key.installImageOverride)
	if errors.Is(err, repo.ErrNoChartName) || apierrors.IsNotFound(err) {
		logrus.Errorf("Failed to find system chart %s, will retry: %v", key.chartName, err)
	} else if err != nil {
		logrus.Errorf("Failed to install system chart %s (release name: %s), will retry: %v", key.chartName, key.releaseName, err)
	}
	return err
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

// Ensure records the chart as desired and, if it is new or has changed, queues it to be installed. It does not block
// on the install.
func (m *Manager) Ensure(namespace, chartName, releaseName, minVersion, exactVersion string, values map[string]any, takeOwnership bool, installImageOverride string) error {
	chart := desired{
		key: desiredKey{
			namespace:            namespace,
			chartName:            chartName,
			releaseName:          releaseName,
			minVersion:           minVersion,
			exactVersion:         exactVersion,
			installImageOverride: installImageOverride,
		},
		values: desiredValues{
			Values:        values,
			TakeOwnership: takeOwnership,
		},
	}
	key := releaseKey{namespace: namespace, releaseName: releaseName}

	m.desiredMu.Lock()
	defer m.desiredMu.Unlock()

	if existing, ok := m.desiredCharts[key]; ok && existing.key == chart.key && equality.Semantic.DeepEqual(existing.values, chart.values) {
		return nil
	}

	m.desiredCharts[key] = chart
	m.queueFor(key).Add(key)
	return nil
}

func (m *Manager) Remove(namespace, releaseName string) {
	m.desiredMu.Lock()
	defer m.desiredMu.Unlock()

	delete(m.desiredCharts, releaseKey{namespace: namespace, releaseName: releaseName})
}

// install tries to install a new version of a chart.
// If the exact version is provided, it will try to install it regardless of whether minVersion is provided.
// If minVersion is provided on its own, it will try to install it only if the current version is earlier than
// minVersion.
// A failure to find a chart for a provided version leads to an error being thrown, without any change in state.
// If no version is provided, it will try to install the latest version available.
// If a release with the version to be installed is already installed, or is pending install, upgrade or rollback, this
// does nothing.
func (m *Manager) install(namespace, chartName, releaseName, minVersion, exactVersion string, values map[string]any, takeOwnership bool, installImageOverride string) error {
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
		// This prevents automated downgrades of updates done manually through the web UI, e.g. for Fleet.
	}

	// This method from the Helm fork doesn't return an error when given a non-existent version, unfortunately.
	// It instead returns the latest version in the index.
	chart, err := index.Get(chartName, v)
	if err != nil {
		// The helm library is using github.com/pkg/errors which is deprecated
		return errors.New(err.Error())
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

	// copy so the caller's (and the desired chart's) values are not mutated
	desiredValue = maps.Clone(desiredValue)
	if desiredValue == nil {
		desiredValue = map[string]any{}
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

	upgrade, err := json.Marshal(types.ChartUpgradeAction{
		Timeout:                &metav1.Duration{Duration: timeoutOrDefault()},
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

// waitPodDone receives an operation, get its pod and check if it's done and
// returns nil if it is. If not, creates a watch for the pod with a timeout of
// settings.SystemManagedChartsOperationTimeout (default 5 minutes) that will
// check if the pod is done and return nil. If the watch timeouts, it returns
// an error. If watchPod returns prematurely due to a watch error, the attempt
// will be continuously retried until the timeout is reached.
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

	timeout := timeoutOrDefault()
	deadline := time.Now().Add(timeout)
	resourceVersion := pod.ResourceVersion

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out after %s waiting for pod %s/%s to complete", timeout, pod.Namespace, pod.Name)
		}

		done, lastResourceVersion, err := m.watchPod(op, resourceVersion, remaining)
		if errors.Is(err, errWatch) {
			pod, err = m.pods.Get(op.Status.PodNamespace, op.Status.PodName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			resourceVersion = lastResourceVersion
			continue
		} else if err != nil {
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

// watchPod opens up a kubernetes watch for a specific pod based on the op's configured pod namespace & name, the
// requested resourceVersion, and timeout. If any watch errors are detected on the response channel, the sentinel error
// errWatch is returned which consumers can use to restart the watch.
func (m *Manager) watchPod(op *catalog.Operation, resourceVersion string, timeout time.Duration) (bool, string, error) {
	sec := int64(timeout.Seconds()) + 1
	resp, err := m.pods.Watch(op.Status.PodNamespace, metav1.ListOptions{
		FieldSelector:   "metadata.name=" + op.Status.PodName,
		ResourceVersion: resourceVersion,
		TimeoutSeconds:  &sec,
	})
	if err != nil {
		return false, "", err
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
		if event.Type == watch.Error {
			return false, resourceVersion, errWatch
		}

		newPod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}

		if newPod.ResourceVersion != resourceVersion {
			resourceVersion = newPod.ResourceVersion
		}

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
func (m *Manager) isInstalled(namespace, name, minVersion, desiredVersion string, isExact bool, desiredValue map[string]any) (bool, string, map[string]any, error) {
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
func desiredVersionAndValues(releases []*release.Release, minVersion, desiredVersion string, isExact bool, desiredValues map[string]any) (bool, string, map[string]any, error) {
	for _, r := range releases {
		if r.Info.Status != releasecommon.StatusDeployed {
			continue
		}
		if desiredValues == nil {
			desiredValues = map[string]any{}
		}
		releaseConfig := r.Config
		if releaseConfig == nil {
			releaseConfig = map[string]any{}
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

		desiredValues = map[string]any{}
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

// timeoutOrDefault returns the settings.SystemManagedChartsOperationTimeout (default of 5 minutes) or 5 minutes if the
// setting cannot be parsed. This function is used for configuring the duration of operation timeouts, namely the
// timeout for an upgrade operation and the timeout for the kubernetes watch for the helm pod's exit status.
func timeoutOrDefault() time.Duration {
	t, err := time.ParseDuration(settings.SystemManagedChartsOperationTimeout.Get())
	if err != nil {
		return defaultTimeout
	}
	return t
}

func getWorkersOrDefault() int {
	w := settings.SystemChartsDefaultWorkers.GetInt()
	if w <= 0 {
		return defaultWorkers
	}
	return w
}
