package hostedcluster

import (
	"context"
	"os"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	cluster2 "github.com/rancher/rancher/pkg/cluster"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	controllerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/kv"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

const priorityClassKey = "priorityClassName"

type operatorChart struct {
	charts  []chart.Definition
	version func() string
}

var (
	AksCrdChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ReleaseName:      "rancher-aks-operator-crd",
		ChartName:        "rancher-aks-operator-crd",
	}
	AksChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ReleaseName:      "rancher-aks-operator",
		ChartName:        "rancher-aks-operator",
	}
	EksCrdChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ReleaseName:      "rancher-eks-operator-crd",
		ChartName:        "rancher-eks-operator-crd",
	}
	EksChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ReleaseName:      "rancher-eks-operator",
		ChartName:        "rancher-eks-operator",
	}
	GkeCrdChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ReleaseName:      "rancher-gke-operator-crd",
		ChartName:        "rancher-gke-operator-crd",
	}
	GkeChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ReleaseName:      "rancher-gke-operator",
		ChartName:        "rancher-gke-operator",
	}

	operatorChartMap = map[string]operatorChart{
		"aks": {
			charts:  []chart.Definition{AksCrdChart, AksChart},
			version: settings.AksOperatorVersion.Get,
		},
		"gke": {
			charts:  []chart.Definition{GkeCrdChart, GkeChart},
			version: settings.GkeOperatorVersion.Get,
		},
		"eks": {
			charts:  []chart.Definition{EksCrdChart, EksChart},
			version: settings.EksOperatorVersion.Get,
		},
	}
)

type handler struct {
	manager      chart.Manager
	projectCache controllerv3.ProjectCache
	secretsCache v1.SecretCache
	chartsConfig chart.RancherConfigGetter
	apps         catalogcontrollers.AppController
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	h := &handler{
		manager:      wContext.SystemChartsManager,
		projectCache: wContext.Mgmt.Project().Cache(),
		secretsCache: wContext.Core.Secret().Cache(),
		chartsConfig: chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
		apps:         wContext.Catalog.App(),
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "cluster-provisioning-operator", h.onClusterChange)
	wContext.Core.Secret().OnChange(ctx, "watch-helm-release", h.onSecretChange)
	wContext.Mgmt.Setting().OnChange(ctx, "hosted-operator-redeploy", h.onSettingsChange)
}

func (h handler) onClusterChange(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}
	skipChartInstallation := strings.EqualFold(settings.SkipHostedClusterChartInstallation.Get(), "true")
	if skipChartInstallation {
		logrus.Warn("Skipping installation of hosted cluster charts, 'skip-hosted-cluster-chart-installation' is set to true")
		return cluster, nil
	}
	switch {
	case cluster.Spec.AKSConfig != nil:
		operator := operatorChartMap["aks"]
		err := h.ensureChart(&operator.charts[0], &operator.charts[1], operator.version())
		if err != nil {
			return cluster, err
		}
	case cluster.Spec.EKSConfig != nil:
		operator := operatorChartMap["eks"]
		err := h.ensureChart(&operator.charts[0], &operator.charts[1], operator.version())
		if err != nil {
			return cluster, err
		}
	case cluster.Spec.GKEConfig != nil:
		operator := operatorChartMap["gke"]
		err := h.ensureChart(&operator.charts[0], &operator.charts[1], operator.version())
		if err != nil {
			return cluster, err
		}
	default:
		return cluster, nil
	}
	return cluster, nil
}

func (h handler) onSettingsChange(_ string, setting *v3.Setting) (*v3.Setting, error) {
	if setting == nil || (setting.Name != settings.SystemDefaultRegistryPullSecrets.Name && setting.Name != settings.SystemDefaultRegistry.Name) {
		return setting, nil
	}
	skipChartInstallation := strings.EqualFold(settings.SkipHostedClusterChartInstallation.Get(), "true")
	if skipChartInstallation {
		logrus.Warn("Skipping installation of hosted cluster charts, 'skip-hosted-cluster-chart-installation' is set to true")
		return setting, nil
	}
	// Ensure that previously installed operators are updated with the latest configuration.
	for provider, operator := range operatorChartMap {
		_, err := h.apps.Cache().Get(operator.charts[1].ReleaseNamespace, operator.charts[1].ReleaseName)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		// update the chart with the latest registry configuration
		err = h.ensureChart(&operator.charts[0], &operator.charts[1], operator.version())
		if err != nil {
			return nil, err
		}
		logrus.Infof("Successfully redeployed %s operator chart due to change in registry settings", provider)
	}
	return setting, nil
}

func (h handler) ensureChart(toInstallCrdChart, toInstallChart *chart.Definition, chartVersion string) error {
	var pullSecrets []string
	registry, _ := cluster2.GetPrivateRegistry(nil)
	if registry != nil {
		pullSecrets = registry.PullSecretNamesAsSlice()
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
			"imagePullSecrets":      pullSecrets,
		},
	}

	additionalCA, err := getAdditionalCA(h.secretsCache)
	if err != nil {
		return err
	}

	chartValues := map[string]interface{}{
		"global":               systemGlobalRegistry,
		"httpProxy":            os.Getenv("HTTP_PROXY"),
		"httpsProxy":           os.Getenv("HTTPS_PROXY"),
		"noProxy":              os.Getenv("NO_PROXY"),
		"additionalTrustedCAs": additionalCA != nil,
	}

	// add priority class value
	if priorityClassName, err := h.chartsConfig.GetGlobalValue(chart.PriorityClassKey); err != nil {
		if !chart.IsNotFoundError(err) {
			logrus.Warnf("Failed to get rancher priorityClassName for '%q': %v", toInstallChart.ChartName, err)
		}
	} else {
		chartValues[priorityClassKey] = priorityClassName
	}

	if err := h.manager.Ensure(
		toInstallCrdChart.ReleaseNamespace,
		toInstallCrdChart.ChartName,
		toInstallCrdChart.ReleaseName,
		chartVersion,
		"",
		nil,
		true,
		""); err != nil {
		return err
	}

	if err := h.manager.Ensure(
		toInstallChart.ReleaseNamespace,
		toInstallChart.ChartName,
		toInstallChart.ReleaseName,
		chartVersion,
		"",
		chartValues,
		true,
		""); err != nil {
		return err
	}

	return nil
}

// check helm release secrets for aks/eks/gke operator chart, if it has been uninstalled, then remove it in h.manager.desiredChart
// so that we don't automatically redeploy it unless there is an AKS/EKS/GKE cluster triggering it
func (h handler) onSecretChange(key string, obj *corev1.Secret) (*corev1.Secret, error) {
	if obj == nil {
		ns, name := kv.Split(key, "/")
		if ns == namespace.System {
			// the name will follow the format sh.helm.release.v1.rancher-eks-operator-crd.v1
			parts := strings.Split(name, ".")
			if len(parts) == 6 {
				releaseName := parts[4]
				if isOperatorChartRelease(releaseName) {
					h.manager.Remove(ns, releaseName)
				}
			}
		}
	}
	return obj, nil
}

func isOperatorChartRelease(name string) bool {
	switch name {
	case AksCrdChart.ChartName, AksChart.ChartName, EksCrdChart.ChartName, EksChart.ChartName, GkeChart.ChartName, GkeCrdChart.ChartName:
		return true
	}
	return false
}

func getAdditionalCA(secretsCache v1.SecretCache) ([]byte, error) {
	secret, err := secretsCache.Get(namespace.System, "tls-ca-additional")
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if secret == nil {
		return nil, nil
	}

	return secret.Data["ca-additional.pem"], nil
}
