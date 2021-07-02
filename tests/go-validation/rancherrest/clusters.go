package rancherrest

import (
	"context"
	"fmt"
	"strings"
	"time"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/clientset/versioned/typed/provisioning.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Cluster struct {
	v1.ClusterInterface
}

type ClusterConfig struct {
	*apisV1.Cluster
}

func NewClusterConfig(clusterName, namespace, cni, machinePoolConfigName, cloudCredentialSecretName string, machinePools []apisV1.RKEMachinePool) *ClusterConfig {
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
		KubernetesVersion:                    "v1.21.2+rke2r1",
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

func (p *ProvisioningV1Client) NewCluster(namespace string) *Cluster {
	clusters := p.Clusters(namespace)
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

func (c *Cluster) CheckClusterStatus(clusterName string) (bool, error) {
	timeout := time.After(6 * time.Minute)
	tick := time.Tick(1 * time.Second)
	ready := false

	for !ready {
		select {
		case <-timeout:
			return false, fmt.Errorf("there was a timeout, and cluster is still provisiioniong")
		case <-tick:
			result, err := c.GetCluster(clusterName)
			if err != nil {
				return false, fmt.Errorf("CLuster get error %v", err)
			}

			for _, condition := range result.Status.Conditions {
				if strings.Contains(condition.Reason, "Error") {
					return false, fmt.Errorf("there was an error %s", condition.Message)
				}
			}
			ready = result.Status.Ready

		}
	}
	return ready, nil
}
