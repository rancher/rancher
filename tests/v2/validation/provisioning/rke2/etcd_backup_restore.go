package rke2

import (
	"fmt"
	"time"

	"github.com/rancher/norman/types"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/sirupsen/logrus"
	appv1 "k8s.io/api/apps/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	localClusterName             = "local"
	wloadBeforeRestore           = "wload-before-restore"
	ingressName                  = "ingress"
	wloadServiceName             = "wload-service"
	wloadAfterRestore            = "wload-after-restore"
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
)

func createSnapshot(client *rancher.Client, clustername string, generation int, namespace string) error {
	clusterObj, existingSteveAPIObj, err := getProvisioningClusterByName(client, clustername, namespace)
	if err != nil {
		return err
	}

	clusterObj.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
		Generation: generation,
	}

	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResouceType).Update(existingSteveAPIObj, clusterObj)
	if err != nil {
		return err
	}

	return nil
}

func restoreSnapshot(client *rancher.Client, clustername string, name string,
	generation int, restoreconfig string, namespace string) error {

	clusterObj, existingSteveAPIObj, err := getProvisioningClusterByName(client, clustername, namespace)
	if err != nil {
		return err
	}

	clusterObj.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{
		Name:             name,
		Generation:       generation,
		RestoreRKEConfig: restoreconfig,
	}

	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResouceType).Update(existingSteveAPIObj, clusterObj)
	if err != nil {
		return err
	}

	return nil
}

func getSnapshots(client *rancher.Client,
	localClusterID string) ([]v1.SteveAPIObject, error) {

	steveclient, err := client.Steve.ProxyDownstream(localClusterID)
	if err != nil {
		return nil, err
	}
	snapshotSteveObjList, err := steveclient.SteveType("rke.cattle.io.etcdsnapshot").List(&types.ListOpts{})
	if err != nil {
		return nil, err
	}

	return snapshotSteveObjList.Data, nil

}

func createRKE2NodeDriverCluster(client *rancher.Client, provider *Provider, clusterName string, k8sVersion string, namespace string, cni string) (*v1.SteveAPIObject, error) {

	nodeRoles := []machinepools.NodeRoles{
		{
			ControlPlane: true,
			Etcd:         false,
			Worker:       false,
			Quantity:     2,
		},
		{
			ControlPlane: false,
			Etcd:         true,
			Worker:       false,
			Quantity:     3,
		},
		{
			ControlPlane: false,
			Etcd:         false,
			Worker:       true,
			Quantity:     3,
		},
	}

	cloudCredential, err := provider.CloudCredFunc(client)
	if err != nil {
		return nil, err
	}
	generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
	machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

	machineConfigResp, err := client.Steve.SteveType(provider.MachineConfigPoolResourceSteveType).Create(machinePoolConfig)
	if err != nil {
		return nil, err
	}

	machinePools := machinepools.RKEMachinePoolSetup(nodeRoles, machineConfigResp)

	cluster := clusters.NewK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, k8sVersion, machinePools)

	return clusters.CreateK3SRKE2Cluster(client, cluster)

}

func getProvisioningClusterByName(client *rancher.Client, clusterName string, namespace string) (*apisV1.Cluster, *v1.SteveAPIObject, error) {
	clusterObj, err := client.Steve.SteveType(ProvisioningSteveResouceType).ByID(namespace + "/" + clusterName)
	if err != nil {
		return nil, nil, err
	}

	cluster := new(apisV1.Cluster)
	err = v1.ConvertToK8sType(clusterObj, &cluster)
	if err != nil {
		return nil, nil, err
	}

	return cluster, clusterObj, nil
}

func upgradeClusterK8sVersion(client *rancher.Client, clustername string, k8sUpgradedVersion string, namespaceName string) error {
	clusterObj, existingSteveAPIObj, err := getProvisioningClusterByName(client, clustername, namespaceName)
	if err != nil {
		return err
	}

	clusterObj.Spec.KubernetesVersion = k8sUpgradedVersion

	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResouceType).Update(existingSteveAPIObj, clusterObj)
	if err != nil {
		return err
	}

	return nil
}

func createDeployment(deployment *appv1.Deployment, steveclient *v1.Client, client *rancher.Client, clusterID string) (*v1.SteveAPIObject, error) {
	deploymentResp2, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	logrus.Infof("created a deployment(%v, deployment).............", deployment.Name)

	logrus.Infof("creating watch over w2.............")
	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		steveclient, err := client.Steve.ProxyDownstream(clusterID)
		if err != nil {
			return false, nil
		}
		deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).ByID(deployment.Namespace + "/" + deployment.Name)
		if err != nil {
			return false, nil
		}
		deployment := &appv1.Deployment{}
		err = v1.ConvertToK8sType(deploymentResp.JSONResp, deployment)
		if err != nil {
			return false, nil
		}
		if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
			return true, nil
		}
		return false, nil
	})
	return deploymentResp2, err

}
