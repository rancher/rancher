package networkpolicy

import (
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

const netPolAnnotation = "networking.management.cattle.io/enable-network-policy"

/*
clusterNetAnnHandler syncs cluster.Spec.EnableNetworkPolicy to cluster.Annotations[netPolAnnotation]
All network policy controllers read from this annotation value to decide if network policy is enabled
*/
type clusterNetAnnHandler struct {
	clusters         v3.ClusterInterface
	clusterNamespace string
}

func (cn *clusterNetAnnHandler) Sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil ||
		cluster.Name != cn.clusterNamespace ||
		!v32.ClusterConditionProvisioned.IsTrue(cluster) {
		return nil, nil
	}

	if cluster.Spec.EnableNetworkPolicy == nil {
		return nil, nil
	}

	if *cluster.Spec.EnableNetworkPolicy == convert.ToBool(cluster.Annotations[netPolAnnotation]) {
		return nil, nil
	}

	logrus.Infof("clusterNetAnnHandler: updating EnableNetworkPolicy of cluster %s to %v", cluster.Name,
		*cluster.Spec.EnableNetworkPolicy)

	cluster.Annotations[netPolAnnotation] = convert.ToString(*cluster.Spec.EnableNetworkPolicy)

	if _, err := cn.clusters.Update(cluster); err != nil {
		return nil, err
	}

	return nil, nil
}
