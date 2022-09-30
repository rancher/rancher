package clusters

import (
	"context"
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	provisioning "github.com/rancher/rancher/tests/framework/clients/rancher/generated/provisioning/v1"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

// GetClusterIDByName is a helper function that returns the cluster ID by name
func GetClusterIDByName(client *rancher.Client, clusterName string) (string, error) {
	clusterList, err := client.Management.Cluster.List(&types.ListOpts{})
	if err != nil {
		return "", err
	}

	for _, cluster := range clusterList.Data {
		if cluster.Name == clusterName {
			return cluster.ID, nil
		}
	}

	return "", nil
}

// GetClusterNameByID is a helper function that returns the cluster ID by name
func GetClusterNameByID(client *rancher.Client, clusterID string) (string, error) {
	clusterList, err := client.Management.Cluster.List(&types.ListOpts{})
	if err != nil {
		return "", err
	}

	for _, cluster := range clusterList.Data {
		if cluster.ID == clusterID {
			return cluster.Name, nil
		}
	}

	return "", nil
}

// IsProvisioningClusterReady is basic check function that would be used for the wait.WatchWait func in pkg/wait.
// This functions just waits until a cluster becomes ready.
func IsProvisioningClusterReady(event watch.Event) (ready bool, err error) {
	cluster := event.Object.(*apisV1.Cluster)
	var updated bool
	ready = cluster.Status.Ready
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == "Updated" && condition.Status == corev1.ConditionTrue {
			updated = true
		}
	}

	return ready && updated, nil
}

// IsHostedProvisioningClusterReady is basic check function that would be used for the wait.WatchWait func in pkg/wait.
// This functions just waits until a hosted cluster becomes ready.
func IsHostedProvisioningClusterReady(event watch.Event) (ready bool, err error) {
	clusterUnstructured := event.Object.(*unstructured.Unstructured)
	cluster := &v3.Cluster{}
	err = scheme.Scheme.Convert(clusterUnstructured, cluster, clusterUnstructured.GroupVersionKind())
	if err != nil {
		return false, err
	}
	for _, cond := range cluster.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			return true, nil
		}
	}

	return false, nil
}

// Verify if a serviceAccountTokenSecret exists or not in the cluster.
func CheckServiceAccountTokenSecret(client *rancher.Client, clusterName string) (success bool, err error) {
	clusterID, err := GetClusterIDByName(client, clusterName)
	if err != nil {
		return false, err
	}

	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return false, err
	}

	if cluster.ServiceAccountTokenSecret != "" {
		logrus.Infof("serviceAccountTokenSecret in this cluster is: %s", cluster.ServiceAccountTokenSecret)
		return true, nil
	} else {
		logrus.Infof("serviceAccountTokenSecret does not exist in this cluster!")
		return false, nil
	}
}

// NewRKE1lusterConfig is a constructor for a v3.Cluster object, to be used by the rancher.Client.Provisioning client.
func NewRKE1ClusterConfig(clusterName, cni, kubernetesVersion string, client *rancher.Client) *management.Cluster {
	clusterConfig := &management.Cluster{
		DockerRootDir:           "/var/lib/docker",
		EnableClusterAlerting:   false,
		EnableClusterMonitoring: false,
		LocalClusterAuthEndpoint: &management.LocalClusterAuthEndpoint{
			Enabled: true,
		},
		Name: clusterName,
		RancherKubernetesEngineConfig: &management.RancherKubernetesEngineConfig{
			DNS: &management.DNSConfig{
				Provider: "coredns",
				Options: map[string]string{
					"stubDomains": "cluster.local",
				},
			},
			Ingress: &management.IngressConfig{
				Provider: "nginx",
			},
			Monitoring: &management.MonitoringConfig{
				Provider: "metrics-server",
			},
			Network: &management.NetworkConfig{
				MTU:     0,
				Options: map[string]string{},
			},
			Version: kubernetesVersion,
		},
	}

	return clusterConfig
}

// NewRKE2ClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredentialSecretName, kubernetesVersion string, machinePools []provisioning.RKEMachinePool) *provisioning.Cluster {
	//metav1.ObjectMeta
	objectMeta := &provisioning.ObjectMeta{
		Name:      clusterName,
		Namespace: namespace,
	}

	etcd := &provisioning.ETCD{
		SnapshotRetention:    5,
		SnapshotScheduleCron: "0 */5 * * *",
	}

	machineGlobalConfigMap := &provisioning.MachineGlobalConfig{
		CNI: cni,
	}

	localClusterAuthEndpoint := &provisioning.LocalClusterAuthEndpoint{
		CACerts: "",
		Enabled: false,
		FQDN:    "",
	}

	upgradeStrategy := &provisioning.ClusterUpgradeStrategy{
		ControlPlaneConcurrency:  "10%",
		ControlPlaneDrainOptions: nil,
		WorkerConcurrency:        "10%",
		WorkerDrainOptions:       nil,
	}

	rkeConfig := &provisioning.RKEConfig{
		MachineGlobalConfig:   machineGlobalConfigMap,
		ETCD:                  etcd,
		UpgradeStrategy:       upgradeStrategy,
		MachineSelectorConfig: []provisioning.RKESystemConfig{},
		MachinePools:          machinePools,
	}

	spec := &provisioning.ClusterSpec{
		CloudCredentialSecretName: cloudCredentialSecretName,
		KubernetesVersion:         kubernetesVersion,
		LocalClusterAuthEndpoint:  localClusterAuthEndpoint,

		RKEConfig: rkeConfig,
	}

	v1Cluster := &provisioning.Cluster{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}

	return v1Cluster
}

// CreateRKE1Cluster is a "helper" functions that takes a rancher client, and the rke1 cluster config as parameters. This function
// registers a delete cluster fuction with a wait.WatchWait to ensure the cluster is removed cleanly.
func CreateRKE1Cluster(client *rancher.Client, rke1Cluster *management.Cluster) (*management.Cluster, error) {
	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=",
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}

	cluster, err := client.Management.Cluster.Create(rke1Cluster)
	if err != nil {
		return nil, err
	}

	client, err = client.ReLogin()
	if err != nil {
		return nil, err
	}

	client.Session.RegisterCleanupFunc(func() error {
		err := client.Management.Cluster.Delete(rke1Cluster)
		if err != nil {
			return err
		}

		adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
		if err != nil {
			return err
		}

		watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, opts)
		if err != nil {
			return err
		}

		return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting cluster")
			} else if event.Type == watch.Deleted {
				return true, nil
			}
			return false, nil
		})
	})

	return cluster, nil
}

// CreateRKE2Cluster is a "helper" functions that takes a rancher client, and the rke2 cluster config as parameters. This function
// registers a delete cluster fuction with a wait.WatchWait to ensure the cluster is removed cleanly.
func CreateRKE2Cluster(client *rancher.Client, rke2Cluster *provisioning.Cluster) (*provisioning.Cluster, error) {
	cluster, err := client.Provisioning.Cluster.Create(rke2Cluster)
	if err != nil {
		return nil, err
	}

	client.Session.RegisterCleanupFunc(func() error {
		adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
		if err != nil {
			return err
		}

		provKubeClient, err := adminClient.GetKubeAPIProvisioningClient()
		if err != nil {
			return err
		}

		watchInterface, err := provKubeClient.Clusters(cluster.ObjectMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + cluster.ObjectMeta.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})

		if err != nil {
			return err
		}

		client, err = client.ReLogin()
		if err != nil {
			return err
		}

		err = client.Provisioning.Cluster.Delete(cluster)
		if err != nil {
			fmt.Println("in cluster cleanup in create")
			return err
		}

		return wait.WatchWait(watchInterface, func(event watch.Event) (ready bool, err error) {
			cluster := event.Object.(*apisV1.Cluster)
			if event.Type == watch.Error {
				return false, fmt.Errorf("there was an error deleting cluster")
			} else if event.Type == watch.Deleted {
				return true, nil
			} else if cluster == nil {
				return true, nil
			}
			return false, nil
		})
	})

	return cluster, nil
}
