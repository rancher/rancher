package certrotation

import (
	"context"

	apiv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/defaults"
	"github.com/rancher/rancher/tests/framework/extensions/provisioning"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace                    = "fleet-default"
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
	machineSteveResourceType     = "cluster.x-k8s.io.machine"
	machineSteveAnnotation       = "cluster.x-k8s.io/machine"
	etcdLabel                    = "node-role.kubernetes.io/etcd"
	clusterLabel                 = "cluster.x-k8s.io/cluster-name"
)

// rotateCerts rotates the certificates in a cluster
func rotateCerts(client *rancher.Client, clusterName string) error {
	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return err
	}

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}

	id, err := clusters.GetV1ProvisioningClusterByName(client, clusterName)
	if err != nil {
		return err
	}

	cluster, err := adminClient.Steve.SteveType(ProvisioningSteveResouceType).ByID(id)
	if err != nil {
		return err
	}

	clusterSpec := &apiv1.ClusterSpec{}
	err = v1.ConvertToK8sType(cluster.Spec, clusterSpec)
	if err != nil {
		return err
	}

	updatedCluster := *cluster
	generation := int64(1)
	if clusterSpec.RKEConfig.RotateCertificates != nil {
		generation = clusterSpec.RKEConfig.RotateCertificates.Generation + 1
	}

	clusterSpec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{
		Generation: generation,
	}

	updatedCluster.Spec = *clusterSpec

	_, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(cluster, updatedCluster)
	if err != nil {
		return err
	}

	logrus.Infof("updated cluster, certs are rotating...")
	kubeRKEClient, err := client.GetKubeAPIRKEClient()
	if err != nil {
		return err
	}

	result, err := kubeRKEClient.RKEControlPlanes(namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	checkFunc := provisioning.CertRotationCompleteCheckFunc(generation)
	logrus.Infof("waiting for certs to rotate, checking status now...")
	err = wait.WatchWait(result, checkFunc)
	if err != nil {
		return err
	}

	clusterWait, err := kubeProvisioningClient.Clusters("fleet-default").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	if err != nil {
		return err
	}

	clusterCheckFunc := clusters.IsProvisioningClusterReady
	logrus.Infof("waiting for cluster to become active again, checking status now...")
	err = wait.WatchWait(clusterWait, clusterCheckFunc)
	if err != nil {
		return err
	}
	return nil
}
