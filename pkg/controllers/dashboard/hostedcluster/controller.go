package hostedcluster

import (
	"context"
	"os"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
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

var (
	AksCrdChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-aks-operator-crd",
	}
	AksChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-aks-operator",
	}
	EksCrdChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-eks-operator-crd",
	}
	EksChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-eks-operator",
	}
	GkeCrdChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-gke-operator-crd",
	}
	GkeChart = chart.Definition{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-gke-operator",
	}
)

type handler struct {
	manager      chart.Manager
	projectCache controllerv3.ProjectCache
	secretsCache v1.SecretCache
	chartsConfig chart.RancherConfigGetter
}

func Register(ctx context.Context, wContext *wrangler.Context) {
	h := &handler{
		manager:      wContext.SystemChartsManager,
		projectCache: wContext.Mgmt.Project().Cache(),
		secretsCache: wContext.Core.Secret().Cache(),
		chartsConfig: chart.RancherConfigGetter{ConfigCache: wContext.Core.ConfigMap().Cache()},
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "cluster-provisioning-operator", h.onClusterChange)
	wContext.Core.Secret().OnChange(ctx, "watch-helm-release", h.onSecretChange)
}

func (h handler) onClusterChange(key string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}

	skipChartInstallation := strings.EqualFold(settings.SkipHostedClusterChartInstallation.Get(), "true")
	if skipChartInstallation {
		logrus.Warn("Skipping installation of hosted cluster charts, 'skip-hosted-cluster-chart-installation' is set to true")
		return cluster, nil
	}

	var toInstallCrdChart, toInstallChart *chart.Definition
	var toInstallCrdChartVersion, toInstallChartVersion string
	toInstallCrdChartVersion = ""
	toInstallChartVersion = ""
	if cluster.Spec.AKSConfig != nil {
		toInstallCrdChart = &AksCrdChart
		toInstallChart = &AksChart
		if aksOperatorVersion := settings.AksOperatorVersion.Get(); aksOperatorVersion != "" {
			toInstallCrdChartVersion = aksOperatorVersion
			toInstallChartVersion = aksOperatorVersion
		}
	} else if cluster.Spec.EKSConfig != nil {
		toInstallCrdChart = &EksCrdChart
		toInstallChart = &EksChart
		if eksOperatorVersion := settings.EksOperatorVersion.Get(); eksOperatorVersion != "" {
			toInstallCrdChartVersion = eksOperatorVersion
			toInstallChartVersion = eksOperatorVersion
		}
	} else if cluster.Spec.GKEConfig != nil {
		toInstallCrdChart = &GkeCrdChart
		toInstallChart = &GkeChart
		if gkeOperatorVersion := settings.GkeOperatorVersion.Get(); gkeOperatorVersion != "" {
			toInstallCrdChartVersion = gkeOperatorVersion
			toInstallChartVersion = gkeOperatorVersion
		}
	}

	if toInstallCrdChart == nil || toInstallChart == nil {
		return cluster, nil
	}

	if err := h.manager.Ensure(
		toInstallCrdChart.ReleaseNamespace,
		toInstallCrdChart.ChartName,
		toInstallCrdChartVersion,
		"",
		nil,
		true,
		""); err != nil {
		return cluster, err
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}

	additionalCA, err := getAdditionalCA(h.secretsCache)
	if err != nil {
		return cluster, err
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
			logrus.Warnf("Failed to get rancher priorityClassName for 'rancher-webhook': %s", err.Error())
		}
	} else {
		chartValues[priorityClassKey] = priorityClassName
	}

	if err := h.manager.Ensure(
		toInstallChart.ReleaseNamespace,
		toInstallChart.ChartName,
		toInstallChartVersion,
		"",
		chartValues,
		true,
		""); err != nil {
		return cluster, err
	}

	return cluster, nil
}

// check helm release secrets for aks/eks/gke operator chart, if it has been uninstalled, then remove it in m.manager.desiredChart
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
