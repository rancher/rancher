package managerancher

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	chartConfigMapName = "chart-contents"
)

var (
	staticFeatures = features.MCM.Name() + "=false," +
		features.MCMAgent.Name() + "=true," +
		features.Fleet.Name() + "=false," +
		features.RKE2.Name() + "=false," +
		features.ProvisioningV2.Name() + "=false," +
		features.EmbeddedClusterAPI.Name() + "=false"
)

type handler struct {
	configMaps   corecontrollers.ConfigMapCache
	settings     mgmtcontrollers.SettingCache
	clusters     rocontrollers.ClusterCache
	mgmtClusters mgmtcontrollers.ClusterCache
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		configMaps:   clients.Core.ConfigMap().Cache(),
		settings:     clients.Mgmt.Setting().Cache(),
		clusters:     clients.Provisioning.Cluster().Cache(),
		mgmtClusters: clients.Mgmt.Cluster().Cache(),
	}
	rocontrollers.RegisterClusterGeneratingHandler(ctx, clients.Provisioning.Cluster(),
		clients.Apply.
			WithSetOwnerReference(false, false).
			WithCacheTypes(clients.Fleet.Bundle(),
				clients.Provisioning.Cluster()),
		"", "manage-rancher", h.OnChange, nil)
	relatedresource.Watch(ctx, "manager-rancher-watch", h.resolveClusters,
		clients.Provisioning.Cluster(), clients.Core.ConfigMap())
}

func (h *handler) resolveClusters(namespace, name string, _ runtime.Object) (result []relatedresource.Key, _ error) {
	if namespace != namespaces.System ||
		name != chartConfigMapName {
		return nil, nil
	}

	clusters, err := h.clusters.List("", labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, cluster := range clusters {
		result = append(result, relatedresource.Key{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		})
	}

	return
}

func (h *handler) values(cluster *rancherv1.Cluster) (map[string]interface{}, error) {
	features, err := h.features(cluster)
	if err != nil {
		return nil, err
	}
	return mergeEnv(data.MergeMaps(map[string]interface{}{
		"systemDefaultRegistry": settings.SystemDefaultRegistry.Get(),
		"ingress": map[string]interface{}{
			"enabled": false,
		},
		"replicas": -1,
		"tls":      "external",
	}, cluster.Spec.RancherValues.Data), features), nil
}

func (h *handler) features(cluster *rancherv1.Cluster) (string, error) {
	monitoringV1 := false
	if cluster.Status.ClusterName != "" {
		mgmtCluster, err := h.mgmtClusters.Get(cluster.Status.ClusterName)
		if err != nil {
			return "", err
		}
		monitoringV1 = mgmtCluster.Spec.EnableClusterMonitoring
	}

	return fmt.Sprintf("%s,%s=%t,%s=%t", staticFeatures,
		features.Legacy.Name(),
		features.Legacy.Enabled(),
		features.MonitoringV1.Name(),
		monitoringV1), nil
}

func mergeEnv(values map[string]interface{}, features string) map[string]interface{} {
	envs := []interface{}{
		map[string]interface{}{
			"name":  "CATTLE_FEATURES",
			"value": features,
		},
		map[string]interface{}{
			"name":  "CATTLE_NO_DEFAULT_ADMIN",
			"value": "true",
		},
	}

	for _, item := range convert.ToInterfaceSlice(values["extraEnv"]) {
		itemMap := convert.ToMapInterface(item)
		switch itemMap["name"] {
		case "CATTLE_FEATURES":
			itemMap["value"] = features + "," + convert.ToString(itemMap["value"])
			envs = envs[1:]
		case "CATTLE_NO_DEFAULT_ADMIN":
			envs[1] = itemMap
			continue
		default:
		}
		envs = append(envs, itemMap)
	}

	return data.MergeMaps(values, map[string]interface{}{
		"extraEnv": envs,
	})
}

func (h *handler) OnChange(cluster *rancherv1.Cluster, status rancherv1.ClusterStatus) ([]runtime.Object, rancherv1.ClusterStatus, error) {
	if cluster.Namespace == "fleet-local" {
		return nil, status, nil
	}

	chart, err := h.configMaps.Get(namespaces.System, chartConfigMapName)
	if apierrors.IsNotFound(err) {
		return nil, status, nil
	} else if err != nil {
		return nil, status, err
	}

	values, err := h.values(cluster)
	if err != nil {
		return nil, status, err
	}

	bundle := &v1alpha1.Bundle{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      name.SafeConcatName(cluster.Name, "managed", "rancher"),
		},
		Spec: v1alpha1.BundleSpec{
			BundleDeploymentOptions: v1alpha1.BundleDeploymentOptions{
				DefaultNamespace: namespaces.System,
				Helm: &v1alpha1.HelmOptions{
					ReleaseName: "rancher",
					Values: &v1alpha1.GenericMap{
						Data: values,
					},
					TakeOwnership: true,
					MaxHistory:    5,
				},
			},
			Resources: nil,
			Targets: []v1alpha1.BundleTarget{
				{
					ClusterName: cluster.Name,
					ClusterSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "provisioning.cattle.io/unmanaged-rancher",
								Operator: metav1.LabelSelectorOpDoesNotExist,
							},
						},
					},
				},
			},
		},
	}

	data := map[string][]byte{}
	if err := json.Unmarshal([]byte(chart.Data["files"]), &data); err != nil {
		return nil, status, err
	}

	for k, v := range data {
		bundle.Spec.Resources = append(bundle.Spec.Resources, v1alpha1.BundleResource{
			Name:    k,
			Content: string(v),
		})
	}

	sort.Slice(bundle.Spec.Resources, func(i, j int) bool {
		return bundle.Spec.Resources[i].Name < bundle.Spec.Resources[j].Name
	})

	return []runtime.Object{
		bundle,
	}, status, nil
}
