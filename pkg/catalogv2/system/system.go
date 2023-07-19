package system

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/content"
	"github.com/rancher/rancher/pkg/catalogv2/helmop"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/settings"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
	namespace string
	name      string
}

type desiredValues struct {
	minVersion           string
	exactVersion         string
	installImageOverride string
	values               map[string]interface{}
	forceAdopt           bool
}

type desired struct {
	key   desiredKey
	value desiredValues
}

type Manager struct {
	ctx                   context.Context
	operation             *helmop.Operations
	content               *content.Manager
	restClientGetter      genericclioptions.RESTClientGetter
	pods                  corecontrollers.PodClient
	desiredCharts         map[desiredKey]desiredValues
	sync                  chan desired
	syncLock              sync.Mutex
	refreshIntervalChange chan struct{}
	settings              mgmtcontrollers.SettingController
	trigger               chan struct{}
	clusterRepos          catalogcontrollers.ClusterRepoController
}

func NewManager(ctx context.Context,
	restClientGetter genericclioptions.RESTClientGetter,
	contentManager *content.Manager,
	ops *helmop.Operations,
	pods corecontrollers.PodClient,
	settings mgmtcontrollers.SettingController,
	clusterRepos catalogcontrollers.ClusterRepoController) (*Manager, error) {

	m := &Manager{
		ctx:                   ctx,
		operation:             ops,
		content:               contentManager,
		restClientGetter:      restClientGetter,
		pods:                  pods,
		sync:                  make(chan desired, 10),
		desiredCharts:         map[desiredKey]desiredValues{},
		refreshIntervalChange: make(chan struct{}, 1),
		settings:              settings,
		trigger:               make(chan struct{}, 1),
		clusterRepos:          clusterRepos,
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

func (m *Manager) runSync() {
	t := time.NewTicker(getIntervalOrDefault(settings.SystemFeatureChartRefreshSeconds.Get()))
	defer t.Stop()

	for {
		select {
		case <-m.refreshIntervalChange:
			t = time.NewTicker(getIntervalOrDefault(settings.SystemFeatureChartRefreshSeconds.Get()))
		case <-m.ctx.Done():
			return
		case <-m.trigger:
			_ = m.installCharts(m.desiredCharts, true)
		case <-t.C:
			_ = m.installCharts(m.desiredCharts, true)
		case desired := <-m.sync:
			v, exists := m.desiredCharts[desired.key]
			// newly requested or changed
			if !exists || !reflect.DeepEqual(v, desired.value) {
				err := m.installCharts(map[desiredKey]desiredValues{
					desired.key: desired.value,
				}, desired.value.forceAdopt)
				if err == nil {
					m.desiredCharts[desired.key] = desired.value
				}
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

func (m *Manager) installCharts(charts map[desiredKey]desiredValues, forceAdopt bool) error {
	var errs []error
	for key, value := range charts {
		for {
			if err := m.install(key.namespace, key.name, value.minVersion, value.exactVersion, value.values, forceAdopt, value.installImageOverride); err == repo.ErrNoChartName || apierrors.IsNotFound(err) {
				logrus.Errorf("Failed to find system chart %s will try again in 5 seconds: %v", key.name, err)
				time.Sleep(5 * time.Second)
				continue
			} else if err != nil {
				logrus.Errorf("Failed to install system chart %s: %v", key.name, err)
				errs = append(errs, err)
			}
			break
		}
	}
	return merr.NewErrors(errs...)
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

func (m *Manager) Ensure(namespace, name, minVersion, exactVersion string, values map[string]interface{}, forceAdopt bool, installImageOverride string) error {
	go func() {
		m.sync <- desired{
			key: desiredKey{
				namespace: namespace,
				name:      name,
			},
			value: desiredValues{
				minVersion:           minVersion,
				exactVersion:         exactVersion,
				installImageOverride: installImageOverride,
				values:               values,
				forceAdopt:           forceAdopt,
			},
		}
	}()
	return nil
}

func (m *Manager) Remove(namespace, name string) {
	for k := range m.desiredCharts {
		if k.namespace == namespace && k.name == name {
			delete(m.desiredCharts, k)
		}
	}
}

func (m *Manager) install(namespace, name, minVersion, exactVersion string, values map[string]interface{}, forceAdopt bool, installImageOverride string) error {
	index, err := m.content.Index("", "rancher-charts", true)
	if err != nil {
		return err
	}

	v := ">=0-a" // latest - this is special syntax to match everything including pre-releases build
	var isExact bool
	if exactVersion != "" {
		v = exactVersion
		isExact = true
	}
	// This method from the Helm fork doesn't return an error when given a non-existent version, unfortunately.
	// It instead returns the latest version in the index.
	chart, err := index.Get(name, v)
	if err != nil {
		return err
	}
	// Because of the behavior of `index.Get`, we need this check.
	if exactVersion != "" && chart.Version != exactVersion {
		return fmt.Errorf("specified exact version %s doesn't exist in the index", exactVersion)
	}

	installed, desiredVersion, desiredValue, err := m.isInstalled(namespace, name, minVersion, chart.Version, isExact, values)
	if err != nil {
		return err
	} else if installed {
		return nil
	}

	if ok, err := m.hasStatus(namespace, name, action.ListPendingInstall); err != nil {
		return err
	} else if ok {
		if err = m.Uninstall(namespace, name); err != nil {
			return err
		}
	}

	upgrade, err := json.Marshal(types.ChartUpgradeAction{
		Timeout:    &metav1.Duration{Duration: 5 * time.Minute},
		Wait:       true,
		Install:    true,
		MaxHistory: 5,
		Namespace:  namespace,
		ForceAdopt: forceAdopt,
		Charts: []types.ChartUpgrade{
			{
				ChartName:   name,
				Version:     desiredVersion,
				ReleaseName: name,
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

	sec := int64(60)
	resp, err := m.pods.Watch(op.Status.PodNamespace, metav1.ListOptions{
		FieldSelector:   "metadata.name=" + pod.Name,
		ResourceVersion: pod.ResourceVersion,
		TimeoutSeconds:  &sec,
	})
	if err != nil {
		return err
	}
	defer func() {
		go func() {
			for range resp.ResultChan() {
			}
		}()
		resp.Stop()
	}()

	for event := range resp.ResultChan() {
		newPod, ok := event.Object.(*v1.Pod)
		if !ok {
			continue
		}
		if ok, err := podDone(op.Status.Chart, newPod); err != nil {
			return err
		} else if ok {
			return nil
		}
	}

	return fmt.Errorf("pod %s/%s failed, watch closed", pod.Namespace, pod.Name)
}

func podDone(chart string, newPod *corev1.Pod) (bool, error) {
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

func (m *Manager) isInstalled(namespace, name, minVersion, desiredVersion string, isExact bool, desiredValue map[string]interface{}) (bool, string, map[string]interface{}, error) {
	helmcfg := &action.Configuration{}
	if err := helmcfg.Init(m.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return false, "", nil, err
	}

	l := action.NewList(helmcfg)
	l.Filter = "^" + name + "$"

	releases, err := l.Run()
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
		if r.Info.Status != release.StatusDeployed {
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
			if !current.Equal(desired) {
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

func (m *Manager) hasStatus(namespace, name string, stateMask action.ListStates) (bool, error) {
	helmcfg := &action.Configuration{}
	if err := helmcfg.Init(m.restClientGetter, namespace, "", logrus.Infof); err != nil {
		return false, err
	}

	l := action.NewList(helmcfg)
	l.Filter = "^" + name + "$"
	l.StateMask = stateMask

	releases, err := l.Run()
	if err != nil {
		return false, err
	}

	return len(releases) != 0, nil
}
