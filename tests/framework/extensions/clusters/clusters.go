package clusters

import (
	"context"
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
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

// IsProvisioningClusterReady is basic check function that would be used for the wait.WatchWait func in pkg/wait.
// This functions just waits until a cluster becomes ready.
func IsProvisioningClusterReady(event watch.Event) (ready bool, err error) {
	cluster := event.Object.(*apisV1.Cluster)

	ready = cluster.Status.Ready
	return ready, nil
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

func RKE1AppendRandomString(baseClusterName string) string {
	const defaultRandStringLength = 5
	clusterName := "auto-" + baseClusterName + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	return clusterName
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
		},
	}

	return clusterConfig
}

// NewRKE2ClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredentialSecretName, kubernetesVersion string, machinePools []apisV1.RKEMachinePool) *apisV1.Cluster {
	typeMeta := metav1.TypeMeta{
		Kind:       "Cluster",
		APIVersion: "provisioning.cattle.io/v1",
	}

	//metav1.ObjectMeta
	objectMeta := metav1.ObjectMeta{
		Name:      clusterName,
		Namespace: namespace,
	}

	etcd := &rkev1.ETCD{
		SnapshotRetention:    5,
		SnapshotScheduleCron: "0 */5 * * *",
	}

	chartValuesMap := rkev1.GenericMap{
		Data: map[string]interface{}{},
	}

	machineGlobalConfigMap := rkev1.GenericMap{
		Data: map[string]interface{}{
			"cni":                 cni,
			"disable-kube-proxy":  false,
			"etcd-expose-metrics": false,
			"profile":             nil,
		},
	}

	localClusterAuthEndpoint := rkev1.LocalClusterAuthEndpoint{
		CACerts: "",
		Enabled: false,
		FQDN:    "",
	}

	upgradeStrategy := rkev1.ClusterUpgradeStrategy{
		ControlPlaneConcurrency:  "10%",
		ControlPlaneDrainOptions: rkev1.DrainOptions{},
		WorkerConcurrency:        "10%",
		WorkerDrainOptions:       rkev1.DrainOptions{},
	}

	rkeSpecCommon := rkev1.RKEClusterSpecCommon{
		ChartValues:           chartValuesMap,
		MachineGlobalConfig:   machineGlobalConfigMap,
		ETCD:                  etcd,
		UpgradeStrategy:       upgradeStrategy,
		MachineSelectorConfig: []rkev1.RKESystemConfig{},
	}

	rkeConfig := &apisV1.RKEConfig{
		RKEClusterSpecCommon: rkeSpecCommon,
		MachinePools:         machinePools,
	}

	spec := apisV1.ClusterSpec{
		CloudCredentialSecretName: cloudCredentialSecretName,
		KubernetesVersion:         kubernetesVersion,
		LocalClusterAuthEndpoint:  localClusterAuthEndpoint,

		RKEConfig: rkeConfig,
	}

	v1Cluster := &apisV1.Cluster{
		TypeMeta:   typeMeta,
		ObjectMeta: objectMeta,
		Spec:       spec,
	}

	return v1Cluster
}

// CreateRKE2Cluster is a "helper" functions that takes a rancher client, and the rke2 cluster config as parameters. This function
// registers a delete cluster fuction with a wait.WatchWait to ensure the cluster is removed cleanly.
func CreateRKE2Cluster(client *rancher.Client, rke2Cluster *apisV1.Cluster) (*apisV1.Cluster, error) {
	cluster, err := client.Provisioning.Clusters(rke2Cluster.Namespace).Create(context.TODO(), rke2Cluster, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	client.Session.RegisterCleanupFunc(func() error {
		err := client.Provisioning.Clusters(rke2Cluster.Namespace).Delete(context.TODO(), rke2Cluster.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
		if err != nil {
			return err
		}

		watchInterface, err := adminClient.Provisioning.Clusters(rke2Cluster.Namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + rke2Cluster.GetName(),
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})

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
