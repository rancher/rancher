// Package webhookchart installs the rancher-webhook chart early in the dashboard controller
// startup so that its WebhookConfigurations and ServiceAccount are owned by Rancher rather than
// by the rancher-webhook ServiceAccount itself. This breaks the chicken-and-egg between
// systemcharts (which transitively writes Settings the webhook validates) and the webhook chart.
package webhookchart

import (
	"context"
	"fmt"
	"sync"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	clusterutil "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/features"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/data"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const repoName = "rancher-charts"

var watchedSettings = map[string]struct{}{
	settings.RancherWebhookVersion.Name: {},
	settings.SystemDefaultRegistry.Name: {},
}

// Register subscribes to changes that affect the webhook chart and ensures the chart is installed.
// Called BEFORE systemcharts.Register so the WebhookConfigurations exist before any other system
// chart install path can transitively touch a resource the webhook validates.
func Register(ctx context.Context, wContext *wrangler.Context, registryOverride string) error {
	h := &handler{
		manager:          wContext.SystemChartsManager,
		chartsConfig:     chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
		registryOverride: registryOverride,
		clusterRepo:      wContext.Catalog.ClusterRepo(),
		clusterCache:     wContext.Mgmt.Cluster().Cache(),
		clusters:         wContext.Mgmt.Cluster(),
	}

	wContext.Catalog.ClusterRepo().OnChange(ctx, "bootstrap-webhook-chart", h.onRepo)
	relatedresource.WatchClusterScoped(ctx, "bootstrap-webhook-chart-settings", relatedSettings,
		wContext.Catalog.ClusterRepo(), wContext.Mgmt.Setting())
	relatedresource.WatchClusterScoped(ctx, "bootstrap-webhook-chart-features", relatedFeatures,
		wContext.Catalog.ClusterRepo(), wContext.Mgmt.Feature())
	wContext.Mgmt.Cluster().OnChange(ctx, "bootstrap-webhook-chart-cluster", h.onCluster)

	return nil
}

type handler struct {
	sync.Mutex
	manager          chart.Manager
	chartsConfig     chart.RancherConfigGetter
	registryOverride string
	clusterRepo      catalogcontrollers.ClusterRepoController
	clusterCache     mgmtcontrollers.ClusterCache
	clusters         mgmtcontrollers.ClusterController
}

func (h *handler) onRepo(_ string, repo *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	if repo == nil || repo.Name != repoName {
		return repo, nil
	}

	h.Lock()
	defer h.Unlock()

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}
	if h.registryOverride != "" {
		// when registryOverride is in effect, the chart's image.repository is rewritten below;
		// blank out systemDefaultRegistry to avoid double-prefixing.
		registryMap := systemGlobalRegistry["cattle"].(map[string]interface{})
		registryMap["systemDefaultRegistry"] = ""
		systemGlobalRegistry["cattle"] = registryMap
	}

	values := map[string]interface{}{
		"global": systemGlobalRegistry,
		// `capi` is no longer used by the webhook chart but legacy values can still surface in
		// `helm get values -n cattle-system rancher-webhook`. Set to nil to clear.
		"capi": nil,
		"mcm": map[string]interface{}{
			"enabled": features.MCM.Enabled(),
		},
	}

	var installImageOverride string
	if h.registryOverride != "" {
		imageSettings, ok := values["image"].(map[string]interface{})
		if !ok {
			imageSettings = map[string]interface{}{}
		}
		imageSettings["repository"] = h.registryOverride + "/rancher/rancher-webhook"
		values["image"] = imageSettings
		installImageOverride = h.registryOverride + "/" + settings.ShellImage.Get()
	}

	h.setPriorityClass(values)

	// Merge per-cluster WebhookDeploymentCustomization; ConfigMap values take highest precedence.
	wdc, err := h.getLocalWebhookCustomization()
	if err != nil && !errors.IsNotFound(err) {
		logrus.Warnf("[webhookchart] failed to get local cluster for webhook values: %v", err)
	} else {
		helmValues, err := chart.WebhookHelmValues(wdc)
		if err != nil {
			logrus.Warnf("[webhookchart] failed to build webhook helm values: %v", err)
		} else {
			values = data.MergeMaps(values, helmValues)
		}
	}
	values = data.MergeMaps(values, h.getChartValues(chart.WebhookChartName))

	// takeOwnership=true so helm adopts the existing rancher.cattle.io WebhookConfigurations
	// created by older webhook code on upgrade.
	if err := h.manager.Ensure(
		namespace.System,
		chart.WebhookChartName,
		chart.WebhookChartName,
		"",
		settings.RancherWebhookVersion.Get(),
		values,
		true,
		installImageOverride,
	); err != nil {
		return repo, fmt.Errorf("failed to install rancher-webhook chart: %w", err)
	}

	if err := h.updateAppliedWebhookCustomization(); err != nil {
		// Log but do not block chart reconciliation — status is best-effort.
		logrus.Warnf("[webhookchart] failed to update applied webhook customization status: %v", err)
	}

	return repo, nil
}

// onCluster re-enqueues the rancher-charts ClusterRepo when the local cluster's
// WebhookDeploymentCustomization drifts from the applied state so the chart is
// re-installed with the updated values.
func (h *handler) onCluster(_ string, obj *v3.Cluster) (*v3.Cluster, error) {
	if obj == nil || obj.DeletionTimestamp != nil || obj.Name != "local" {
		return obj, nil
	}
	if clusterutil.WebhookDeploymentCustomizationChanged(obj) {
		h.clusterRepo.EnqueueAfter(repoName, 2*time.Second)
	}
	return obj, nil
}

// getLocalWebhookCustomization returns the WebhookDeploymentCustomization from the local
// management cluster, or nil if the cluster has no customization set.
// On downstream clusters (MCMAgent enabled), webhook customization is delivered via the
// rancher-config ConfigMap pushed by the management server's clusterdeploy controller;
// getChartValues() picks those up, so this returns nil in that context.
func (h *handler) getLocalWebhookCustomization() (*v3.WebhookDeploymentCustomization, error) {
	if features.MCMAgent.Enabled() {
		return nil, nil
	}
	cluster, err := h.clusterCache.Get("local")
	if err != nil {
		return nil, err
	}
	return cluster.Spec.WebhookDeploymentCustomization, nil
}

// updateAppliedWebhookCustomization copies the local cluster's WebhookDeploymentCustomization
// from Spec to Status so drift detection can compare applied vs desired state.
// No-op on downstream clusters where the management server tracks applied state.
func (h *handler) updateAppliedWebhookCustomization() error {
	if features.MCMAgent.Enabled() {
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cached, err := h.clusterCache.Get("local")
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		cluster := cached.DeepCopy()
		clusterutil.UpdateAppliedWebhookDeploymentCustomization(cluster)
		// Use Update (not UpdateStatus) because the clusters.management.cattle.io
		// CRD does not define a status subresource.
		_, err = h.clusters.Update(cluster)
		return err
	})
}

func (h *handler) setPriorityClass(values map[string]interface{}) {
	priorityClassName, err := h.chartsConfig.GetGlobalValue(chart.PriorityClassKey)
	if err == nil {
		values[chart.PriorityClassKey] = priorityClassName
	} else if !chart.IsNotFoundError(err) {
		logrus.Warnf("[webhookchart] failed to get %s for %s: %s", chart.PriorityClassKey, chart.WebhookChartName, err.Error())
	}
}

func (h *handler) getChartValues(chartName string) map[string]interface{} {
	configMapValues, err := h.chartsConfig.GetChartValues(chartName)
	if err != nil && !chart.IsNotFoundError(err) {
		logrus.Warnf("[webhookchart] failed to get chart values for %s: %s", chartName, err.Error())
	}
	return configMapValues
}

func relatedSettings(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if s, ok := obj.(*v3.Setting); ok {
		if _, ok := watchedSettings[s.Name]; ok {
			return []relatedresource.Key{{Name: repoName}}, nil
		}
	}
	return nil, nil
}

func relatedFeatures(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if _, ok := obj.(*v3.Feature); ok {
		return []relatedresource.Key{{Name: repoName}}, nil
	}
	return nil, nil
}
