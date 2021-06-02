package hostedkubernetecharts

import (
	"context"
	"os"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalogv2/system"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

var (
	AksCrdChart = chartDef{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-aks-operator-crd",
	}
	AksChart = chartDef{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-aks-operator",
	}
	EksCrdChart = chartDef{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-eks-operator-crd",
	}
	EksChart = chartDef{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-eks-operator",
	}
	GkeCrdChart = chartDef{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-gke-operator-crd",
	}
	GkeChart = chartDef{
		ReleaseNamespace: "cattle-system",
		ChartName:        "rancher-gke-operator",
	}
)

type chartDef struct {
	ReleaseNamespace string
	ChartName        string
}

type handler struct {
	manager *system.Manager
}

func Register(ctx context.Context, wContext *wrangler.Context) error {
	h := &handler{
		manager: wContext.SystemChartsManager,
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "cluster-provisioning-operator", h.onClusterChange)

	return nil
}

func (h handler) onClusterChange(key string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	var toInstallCrdChart, toInstallChart *chartDef
	var minVersion string
	if cluster.Spec.AKSConfig != nil {
		toInstallCrdChart = &AksCrdChart
		toInstallChart = &AksChart
		minVersion = settings.AKSOperatorMinVersion.Get()
	} else if cluster.Spec.EKSConfig != nil {
		toInstallCrdChart = &EksCrdChart
		toInstallChart = &EksChart
		minVersion = settings.EKSOperatorMinVersion.Get()
	} else if cluster.Spec.GKEConfig != nil {
		toInstallCrdChart = &GkeCrdChart
		toInstallChart = &GkeChart
		minVersion = settings.GKEOperatorMinVersion.Get()
	}

	if toInstallCrdChart == nil || toInstallChart == nil {
		return cluster, nil
	}

	if err := h.manager.Ensure(toInstallCrdChart.ReleaseNamespace, toInstallCrdChart.ChartName, minVersion, nil, true); err != nil {
		return cluster, err
	}

	systemGlobalRegistry := map[string]interface{}{
		"cattle": map[string]interface{}{
			"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		},
	}

	chartValues := map[string]interface{}{
		"global":     systemGlobalRegistry,
		"httpProxy":  os.Getenv("HTTP_PROXY"),
		"httpsProxy": os.Getenv("HTTPS_PROXY"),
		"noProxy":    os.Getenv("NO_PROXY"),
	}

	if err := h.manager.Ensure(toInstallChart.ReleaseNamespace, toInstallChart.ChartName, minVersion, chartValues, true); err != nil {
		return cluster, err
	}

	return cluster, nil
}
