package clusters

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
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

// NewK3SRKE2ClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func NewK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredentialSecretName, kubernetesVersion string, machinePools []apisV1.RKEMachinePool) *apisV1.Cluster {
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

// HardenK3SRKE2ClusterConfig is a constructor for a apisV1.Cluster object, to be used by the rancher.Client.Provisioning client.
func HardenK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredentialSecretName, kubernetesVersion string, machinePools []apisV1.RKEMachinePool) *apisV1.Cluster {
	v1Cluster := NewK3SRKE2ClusterConfig(clusterName, namespace, cni, cloudCredentialSecretName, kubernetesVersion, machinePools)

	v1Cluster.Spec.RKEConfig.MachineGlobalConfig.Data["kube-apiserver-arg"] = []string{
		"enable-admission-plugins=NodeRestriction,PodSecurityPolicy,ServiceAccount",
		"audit-policy-file=/var/lib/rancher/k3s/server/audit.yaml",
		"audit-log-path=/var/lib/rancher/k3s/server/logs/audit.log",
		"audit-log-maxage=30",
		"audit-log-maxbackup=10",
		"audit-log-maxsize=100",
		"request-timeout=300s",
		"service-account-lookup=true",
	}

	v1Cluster.Spec.RKEConfig.MachineSelectorConfig = []rkev1.RKESystemConfig{
		{
			Config: rkev1.GenericMap{
				Data: map[string]interface{}{
					"kubelet-arg": []string{
						"make-iptables-util-chains=true",
					},
					"protect-kernel-defaults": true,
				},
			},
		},
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

// CreateK3SRKE2Cluster is a "helper" functions that takes a rancher client, and the rke2 cluster config as parameters. This function
// registers a delete cluster fuction with a wait.WatchWait to ensure the cluster is removed cleanly.
func CreateK3SRKE2Cluster(client *rancher.Client, rke2Cluster *apisV1.Cluster) (*v1.SteveAPIObject, error) {
	cluster, err := client.Steve.SteveType(ProvisioningSteveResouceType).Create(rke2Cluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		_, err = client.Steve.SteveType(ProvisioningSteveResouceType).ByID(cluster.ID)
		if err != nil {
			return false, err
		} else {
			return true, nil
		}
	})

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

		err = client.Steve.SteveType(ProvisioningSteveResouceType).Delete(cluster)
		if err != nil {
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

// UpdateK3SRKE2Cluster is a "helper" functions that takes a rancher client, old rke2/k3s cluster config, and the new rke2/k3s cluster config as parameters.
func UpdateK3SRKE2Cluster(client *rancher.Client, cluster *v1.SteveAPIObject, updatedCluster *apisV1.Cluster) (*v1.SteveAPIObject, error) {
	updateCluster, err := client.Steve.SteveType(ProvisioningSteveResouceType).ByID(cluster.ID)
	if err != nil {
		return nil, err
	}

	updatedCluster.ObjectMeta.ResourceVersion = updateCluster.ObjectMeta.ResourceVersion

	logrus.Infof("Applying cluster YAML hardening changes...")
	cluster, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(cluster, updatedCluster)
	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 5*time.Minute, func() (done bool, err error) {
		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		clusterResp, err := client.Steve.SteveType(ProvisioningSteveResouceType).ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.ObjectMeta.State.Name == "active" {
			logrus.Infof("Cluster YAML has successfully been updated!")
			return true, nil
		} else {
			return false, nil
		}
	})

	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// WaitForClusterToBeUpgraded is a "helper" functions that takes a rancher client, and the cluster id as parameters. This function
// contains two stages. First stage is to wait to be cluster in upgrade state. And the other is to wait until cluster is ready.
// Cluster error states that declare control plane is inaccessible and cluster object modified are ignored.
// Same cluster summary information logging is ignored.
func WaitClusterToBeUpgraded(client *rancher.Client, clusterID string) (err error) {
	clusterStateUpgrading := "upgrading" // For imported RKE2 and K3s clusters
	clusterStateUpdating := "updating"   // For all clusters except imported K3s and RKE2

	clusterErrorStateMessage := "cluster is in error state"

	var clusterInfo string
	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterID,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}

	watchInterface, err := client.GetManagementWatchInterface(management.ClusterType, opts)
	if err != nil {
		return
	}
	checkFuncWaitToBeInUpgrade := func(event watch.Event) (bool, error) {
		clusterUnstructured := event.Object.(*unstructured.Unstructured)
		summerizedCluster := summary.Summarize(clusterUnstructured)

		clusterInfo = logClusterInfoWithChanges(clusterID, clusterInfo, summerizedCluster)

		if summerizedCluster.Transitioning && !summerizedCluster.Error && (summerizedCluster.State == clusterStateUpdating || summerizedCluster.State == clusterStateUpgrading) {
			return true, nil
		} else if summerizedCluster.Error && isClusterInaccessible(summerizedCluster.Message) {
			return false, nil
		} else if summerizedCluster.Error && !isClusterInaccessible(summerizedCluster.Message) {
			return false, errors.Wrap(err, clusterErrorStateMessage)
		}

		return false, nil
	}
	err = wait.WatchWait(watchInterface, checkFuncWaitToBeInUpgrade)
	if err != nil {
		return
	}

	watchInterfaceWaitUpgrade, err := client.GetManagementWatchInterface(management.ClusterType, opts)
	checkFuncWaitUpgrade := func(event watch.Event) (bool, error) {
		clusterUnstructured := event.Object.(*unstructured.Unstructured)
		summerizedCluster := summary.Summarize(clusterUnstructured)

		clusterInfo = logClusterInfoWithChanges(clusterID, clusterInfo, summerizedCluster)

		if summerizedCluster.IsReady() {
			return true, nil
		} else if summerizedCluster.Error && isClusterInaccessible(summerizedCluster.Message) {
			return false, nil
		} else if summerizedCluster.Error && !isClusterInaccessible(summerizedCluster.Message) {
			return false, errors.Wrap(err, clusterErrorStateMessage)

		}

		return false, nil
	}

	err = wait.WatchWait(watchInterfaceWaitUpgrade, checkFuncWaitUpgrade)
	if err != nil {
		return err
	}

	return
}

func isClusterInaccessible(messages []string) (isInaccessible bool) {
	clusterCPErrorMessage := "Cluster health check failed: Failed to communicate with API server during namespace check" // For GKE
	clusterModifiedErrorMessage := "the object has been modified"                                                        // For provisioning node driver K3s and RKE2

	for _, message := range messages {
		if strings.Contains(message, clusterCPErrorMessage) || strings.Contains(message, clusterModifiedErrorMessage) {
			isInaccessible = true
			break
		}
	}

	return
}

func logClusterInfoWithChanges(clusterID, clusterInfo string, summary summary.Summary) string {
	newClusterInfo := fmt.Sprintf("ClusterID: %v, Message: %v, Error: %v, State: %v, Transiationing: %v", clusterID, summary.Message, summary.Error, summary.State, summary.Transitioning)

	if clusterInfo != newClusterInfo {
		logrus.Infof(newClusterInfo)
		clusterInfo = newClusterInfo
	}

	return clusterInfo
}
