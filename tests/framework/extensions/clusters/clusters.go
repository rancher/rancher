package clusters

import (
	"context"
	"fmt"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// IsProvisioningClusterReady is basic check function that would be used for the wait.WatchWait func in pkg/wait.
// This functions just waits until a cluster becomes ready.
func IsProvisioningClusterReady(event watch.Event) (ready bool, err error) {
	cluster := event.Object.(*apisV1.Cluster)

	ready = cluster.Status.Ready
	return ready, nil
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
	client.Session.RegisterCleanupFunc(func() error {
		err := client.Provisioning.Clusters(rke2Cluster.Namespace).Delete(context.TODO(), rke2Cluster.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		watchInterface, err := client.Provisioning.Clusters(rke2Cluster.Namespace).Watch(context.TODO(), metav1.ListOptions{
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

	return client.Provisioning.Clusters(rke2Cluster.Namespace).Create(context.TODO(), rke2Cluster, metav1.CreateOptions{})
}
