package cluster

import (
	"context"
	"fmt"
	"strings"
	"time"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/go-validation/aws"
	"github.com/rancher/rancher/tests/go-validation/environmentvariables"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var CNI = environmentvariables.Getenv("CNI", "calico")
var KubernetesVersion = environmentvariables.Getenv("KUBERNETES_VERSION", "v1.21.3-rc4+rke2r2")

type NodeConfig struct {
	Name     string
	NumNodes int64
	Roles    []string
}

func NewNodeConfig(name string, numNodes int64, roles []string) *NodeConfig {
	return &NodeConfig{
		Name:     name,
		NumNodes: numNodes,
		Roles:    roles,
	}
}

type Cluster struct {
	v1.ClusterInterface
}

type ClusterConfig struct {
	*apisV1.Cluster
}

func NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredentialSecretName, kubernetesVersion string, machinePools []apisV1.RKEMachinePool) *ClusterConfig {
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
		DisableSnapshots:     false,
		S3:                   nil,
		SnapshotRetention:    5,
		SnapshotScheduleCron: "0 */5 * * *",
	}

	chartValuesMap := rkev1.GenericMap{
		Data: map[string]interface{}{},
	}

	machineGlobalConfigMap := rkev1.GenericMap{
		Data: map[string]interface{}{
			"cni":     cni,
			"profile": nil,
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
		ChartValues:              chartValuesMap,
		MachineGlobalConfig:      machineGlobalConfigMap,
		ETCD:                     etcd,
		LocalClusterAuthEndpoint: localClusterAuthEndpoint,
		UpgradeStrategy:          upgradeStrategy,
		MachineSelectorConfig:    []rkev1.RKESystemConfig{},
	}

	rkeConfig := &apisV1.RKEConfig{
		RKEClusterSpecCommon: rkeSpecCommon,
		MachinePools:         machinePools,
	}

	spec := apisV1.ClusterSpec{
		CloudCredentialSecretName:            cloudCredentialSecretName,
		KubernetesVersion:                    kubernetesVersion,
		DefaultPodSecurityPolicyTemplateName: "",

		RKEConfig: rkeConfig,
	}

	v1Cluster := &apisV1.Cluster{
		TypeMeta:   typeMeta,
		ObjectMeta: objectMeta,
		Spec:       spec,
	}

	return &ClusterConfig{
		v1Cluster,
	}
}

func NewCluster(namespace string, client *v1.ProvisioningV1Client) *Cluster {
	clusters := client.Clusters(namespace)
	return &Cluster{
		clusters,
	}
}

func (c *Cluster) CreateCluster(clusterConfig *ClusterConfig) (*apisV1.Cluster, error) {
	ctx := context.Background()
	v1Cluster, err := c.Create(ctx, clusterConfig.Cluster, metav1.CreateOptions{})

	return v1Cluster, err
}

func (c *Cluster) GetCluster(clusterName string) (*apisV1.Cluster, error) {
	ctx := context.Background()
	v1Cluster, err := c.Get(ctx, clusterName, metav1.GetOptions{})
	return v1Cluster, err
}

func (c *Cluster) DeleteCluster(clusterName string) error {
	ctx := context.Background()
	err := c.Delete(ctx, clusterName, metav1.DeleteOptions{})
	return err
}

func ClusterCleanup(ec2Client *aws.EC2Client, clusters []*Cluster, clusterNames []string, nodes []*aws.EC2Node) error {
	for index, cluster := range clusters {
		clusterName := clusterNames[index]
		err := cluster.DeleteCluster(clusterName)
		if err != nil {
			return err
		}
	}

	if ec2Client != nil {
		_, err := ec2Client.DeleteNodes(nodes)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Cluster) CheckClusterStatus(clusterName string) (bool, error) {
	timeout := time.After(10 * time.Minute)
	tick := time.Tick(1 * time.Second)
	ready := false

	count := 0
	getCount := 0

	for !ready {
		select {
		case <-timeout:
			return false, fmt.Errorf("there was a timeout, and cluster is still provisioning")
		case <-tick:
			result, err := c.GetCluster(clusterName)
			if err != nil {
				getCount += 1
				if getCount == 10 {
					return false, fmt.Errorf("Cluster get error: %v", err)
				}
			}

			for _, condition := range result.Status.Conditions {
				if strings.Contains(condition.Reason, "Error") {
					count += 1
					if count == 10 {
						return false, fmt.Errorf("there was an error: %s", condition.Message)
					}
				}
			}
			ready = result.Status.Ready

		}
	}
	return ready, nil
}

func (c *Cluster) PollCluster(clusterName string) (*apisV1.Cluster, error) {
	timeout := time.After(30 * time.Second)
	tick := time.Tick(1 * time.Second)
	var cluster *apisV1.Cluster
	var err error
	for cluster == nil {
		select {
		case <-timeout:
			return nil, err
		case <-tick:
			cluster, err = c.GetCluster(clusterName)
		}
	}
	return cluster, nil
}
