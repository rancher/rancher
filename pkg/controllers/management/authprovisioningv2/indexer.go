package authprovisioningv2

import (
	"fmt"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/wrangler/pkg/data"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
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

	data, err := data.Convert(obj)
	if err != nil {
		return nil, err
	}
	clusterName := data.String("spec", "clusterName")
	if clusterName != "" {
		return []string{clusterName}, nil
	}

	var result []string
	targets := data.Slice("spec", "target")
	if len(targets) == 1 {
		clusterName := data.String("clusterName")
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
