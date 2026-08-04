package authprovisioningv2

import (
	"fmt"
	"strings"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/capr/dynamicschema"
	"github.com/rancher/wrangler/v3/pkg/data"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type clusterNames interface {
	GetClusterNames() []string
}

func getObjectClusterNames(obj runtime.Object) ([]string, error) {
	clusterNamed, ok := obj.(clusterNames)
	if ok {
		return clusterNamed.GetClusterNames(), nil
	}

	switch o := obj.(type) {
	case *capi.Machine:
		return []string{o.Spec.ClusterName}, nil
	case *capi.Cluster:
		return []string{o.Name}, nil
	case *v1.Cluster:
		return []string{o.Name}, nil
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	objMeta, err := meta.Accessor(obj)
	// If there is an error, skip this block
	if err == nil {
		if gvk.Group == dynamicschema.MachineConfigAPIGroup {
			for _, owner := range objMeta.GetOwnerReferences() {
				if owner.APIVersion == "provisioning.cattle.io/v1" && owner.Kind == "Cluster" {
					return []string{owner.Name}, nil
				}
			}
		}

		// Infrastructure/bootstrap/controlplane provider objects (e.g. AWSMachineTemplate)
		// may have an ownerRef to a cluster.x-k8s.io/Cluster but no spec.clusterName field.
		// Since the CAPI Cluster name equals the provisioning Cluster name, the ownerRef
		// name is directly usable as the cluster name. Match on the group prefix to be
		// version-agnostic (v1beta1, v1beta2, future v1, etc.).
		for _, owner := range objMeta.GetOwnerReferences() {
			if strings.HasPrefix(owner.APIVersion, "cluster.x-k8s.io/") && owner.Kind == "Cluster" {
				return []string{owner.Name}, nil
			}
		}

		// Infrastructure provider objects (e.g. AWSCluster, AWSMachine) carry the
		// CAPI-standard label cluster.x-k8s.io/cluster-name set by CAPI controllers.
		// AWSMachine for example has no spec.clusterName and its ownerRef points to a
		// Machine (not a Cluster directly), so the label is the most reliable fallback.
		if clusterName := objMeta.GetLabels()["cluster.x-k8s.io/cluster-name"]; clusterName != "" {
			return []string{clusterName}, nil
		}
	}

	objData, err := data.Convert(obj)
	if err != nil {
		return nil, err
	}
	clusterName := objData.String("spec", "clusterName")
	if clusterName != "" {
		return []string{clusterName}, nil
	}

	var result []string
	targets := objData.Slice("spec", "target")
	if len(targets) == 1 {
		clusterName := objData.String("clusterName")
		if clusterName != "" {
			result = append(result, clusterName)
		}
	}

	return result, nil
}

func indexByCluster(obj runtime.Object) ([]string, error) {
	clusterNames, err := getObjectClusterNames(obj)
	if err != nil {
		return nil, err
	}

	if len(clusterNames) == 0 {
		return nil, nil
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(clusterNames))
	for _, clusterName := range clusterNames {
		result = append(result, fmt.Sprintf("%s/%s", meta.GetNamespace(), clusterName))
	}

	return result, nil
}
