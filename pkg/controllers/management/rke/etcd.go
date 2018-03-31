package rke

import (
	"fmt"
	"reflect"

	"github.com/rancher/rancher/pkg/clusterprovisioninglogger"
	"github.com/rancher/rancher/pkg/librke"
	"github.com/rancher/rke/cluster"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provisioner) etcd(cluster *v3.Cluster) (*v3.Cluster, error) {
	newSpec, err := p.builder.GetSpec(cluster, true)
	if err != nil {
		return nil, err
	}

	// get two copies of the spec because reconcile tends to modify the copy
	cleanSpec, err := p.builder.GetSpec(cluster, true)
	if err != nil {
		return nil, err
	}

	if cluster.Status.AppliedEtcdSpec != nil && reflect.DeepEqual(cluster.Status.AppliedEtcdSpec.RancherKubernetesEngineConfig, newSpec.RancherKubernetesEngineConfig) {
		return cluster, nil
	}

	// sanity test we have the same spec
	if !reflect.DeepEqual(cleanSpec, newSpec) {
		return nil, fmt.Errorf("need to restart etcd provisioning")
	}

	if err := p.reconcileEtcd(cluster, newSpec); err != nil {
		newObj, _ := p.Clusters.Get(cluster.Name, v1.GetOptions{})
		return newObj, err
	}

	newObj, _ := p.Clusters.Get(cluster.Name, v1.GetOptions{})
	if err != nil {
		return cluster, err
	}

	newObj.Status.AppliedEtcdSpec = cleanSpec
	return p.Clusters.Update(newObj)
}

func (p *Provisioner) reconcileEtcd(cluster *v3.Cluster, newSpec *v3.ClusterSpec) error {
	oldCluster, err := p.builder.ParseCluster(cluster.Name, cluster.Status.AppliedEtcdSpec)
	if err != nil {
		return err
	}

	newCluster, err := p.builder.ParseCluster(cluster.Name, newSpec)
	if err != nil {
		return err
	}

	if newCluster == nil || len(newCluster.Nodes) == 0 {
		return nil
	}

	bundle, err := p.builder.GetOrGenerateCerts(cluster)
	if err != nil {
		return err
	}

	if oldCluster != nil {
		oldCluster.Certificates = bundle.Certs()
		if len(oldCluster.Nodes) == 0 || noOverlap(oldCluster, newCluster) {
			oldCluster = nil
		}
	}
	newCluster.Certificates = bundle.Certs()

	ctx, logger := clusterprovisioninglogger.NewNonRPCLogger(p.Clusters, p.EventLogger, cluster, v3.ClusterConditionEtcd)
	defer logger.Close()

	return librke.New().EtcdUp(ctx, oldCluster, newCluster)
}

func noOverlap(oldCluster, newCluster *cluster.Cluster) bool {
	oldAddress := map[string]bool{}
	for _, node := range oldCluster.EtcdHosts {
		oldAddress[node.Address] = true
	}

	for _, node := range newCluster.EtcdHosts {
		if oldAddress[node.Address] {
			return false
		}
	}

	return true
}
