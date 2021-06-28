package hostedcluster

import (
	"context"
	"os"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/controllers/dashboard/chart"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

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
	manager *system.Manager
	secrets v1.SecretCache
}

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
		secrets: wContext.Core.Secret().Cache(),
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "cluster-provisioning-operator", h.onClusterChange)

	return nil
}

func (h handler) onClusterChange(key string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	var toInstallCrdChart, toInstallChart *chart.Definition
	if cluster.Spec.AKSConfig != nil {
		toInstallCrdChart = &AksCrdChart
		toInstallChart = &AksChart
	} else if cluster.Spec.EKSConfig != nil {
		toInstallCrdChart = &EksCrdChart
		toInstallChart = &EksChart
	} else if cluster.Spec.GKEConfig != nil {
		toInstallCrdChart = &GkeCrdChart
		toInstallChart = &GkeChart
	}

	if toInstallCrdChart == nil || toInstallChart == nil {
		return cluster, nil
	}

	if err := h.manager.Ensure(toInstallCrdChart.ReleaseNamespace, toInstallCrdChart.ChartName, "", nil, true); err != nil {
		return cluster, err
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}

	additionalCA, err := getAdditionalCA(h.secrets)
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

	if err := h.manager.Ensure(toInstallChart.ReleaseNamespace, toInstallChart.ChartName, "", chartValues, true); err != nil {
		return cluster, err
	}

	return cluster, nil
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
