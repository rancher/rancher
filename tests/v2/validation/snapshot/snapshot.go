package snapshot

import (
	"strings"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/etcdsnapshot"
	"github.com/rancher/rancher/tests/framework/extensions/ingresses"
	"github.com/rancher/rancher/tests/framework/extensions/provisioning"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	podV1 "github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	namespace                    = "fleet-default"
	defaultNamespace             = "default"
	localClusterName             = "local"
	initialWorkloadName          = "wload-before-restore"
	initialIngressName           = "ingress-before-restore"
	serviceAppendName            = "service-"
	WorkloadNamePostBackup       = "wload-after-backup"
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
	isCattleLabeled              = true
	maxContainerRestartCount     = 3
	cattleSystem                 = "cattle-system"
	podPrefix                    = "helm-operation"
	containerName                = "nginx"
	containerImage               = "nginx"
	ingressPath                  = "/index.html"
	cpConcurrencyValue           = "15%"
	workerConcurrencyValue       = "20%"
	concurrencyDefaultValue      = "10%"
)

func SnapshotRestore(t *testing.T, client *rancher.Client, clusterName string, upgrade string, strategy bool) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(t, err)
	require.NotEmptyf(t, clusterID, "cluster id is empty")

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(t, err)

	localClusterID, err := clusters.GetClusterIDByName(client, localClusterName)
	require.NoError(t, err)

	deploymentResponse, err := createDeployment(steveclient, initialWorkloadName)
	require.NoError(t, err)

	err = workloads.VerifyDeployment(steveclient, deploymentResponse)
	require.NoError(t, err)
	require.Equal(t, initialWorkloadName, deploymentResponse.ObjectMeta.Name)

	ingressResp, err := createIngress(steveclient, initialIngressName, serviceAppendName+initialWorkloadName)
	require.NoError(t, err)
	require.Equal(t, initialIngressName, ingressResp.ObjectMeta.Name)

	existingSnapshots, err := provisioning.GetSnapshots(client, localClusterID, clusterName)
	require.NoError(t, err)

	etcdNodeCount, _ := MatchNodeToAnyEtcdRole(t, client, clusterID)

	err = etcdsnapshot.CreateSnapshot(client, clusterName)
	require.NoError(t, err)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	clusterObject, _, err := clusters.GetProvisioningClusterByName(adminClient, clusterName, namespace)
	require.NoError(t, err)

	initialKubernetesVersion := clusterObject.Spec.KubernetesVersion
	logrus.Infof("creating kube provisioning client.............")
	kubeProvisioningClient, err := client.GetKubeAPIProvisioningClient()
	require.NoError(t, err)

	err = clusters.WatchAndWaitForCluster(client.Steve, kubeProvisioningClient, "fleet-default", clusterName)
	require.NoError(t, err)

	podResults, podErrors := pods.StatusPods(client, clusterID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)
	require.NoError(t, err)
	require.Equal(t, initialKubernetesVersion, clusterObject.Spec.KubernetesVersion)

	snapshotToRestore, err := provisioning.VerifySnapshots(client, localClusterID, clusterName, etcdNodeCount+len(existingSnapshots))
	require.NoError(t, err)

	deploymentResponse, err = createDeployment(steveclient, WorkloadNamePostBackup)
	require.NoError(t, err)

	err = workloads.VerifyDeployment(steveclient, deploymentResponse)
	require.NoError(t, err)
	require.Equal(t, WorkloadNamePostBackup, deploymentResponse.ObjectMeta.Name)

	if upgrade != "" {
		clusterObject, clusterResponse, err := clusters.GetProvisioningClusterByName(adminClient, clusterName, namespace)
		require.NoError(t, err)
		clusterObject.Spec.KubernetesVersion = upgrade
		if strategy {
			clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency = cpConcurrencyValue
			clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency = workerConcurrencyValue
		}
		_, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(clusterResponse, clusterObject)
		require.NoError(t, err)
		err = clusters.WaitClusterToBeUpgraded(client, clusterID)
		require.NoError(t, err)

		podResults, podErrors = pods.StatusPods(client, clusterID)
		assert.NotEmpty(t, podResults)
		assert.Empty(t, podErrors)
		require.NoError(t, err)
		require.Equal(t, upgrade, clusterObject.Spec.KubernetesVersion)

		if strategy {
			require.Equal(t, cpConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			require.Equal(t, workerConcurrencyValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
		}
	}
	snapshotRestore := &rkev1.ETCDSnapshotRestore{
		Name:             snapshotToRestore,
		Generation:       clusterObject.Spec.RKEConfig.ETCDSnapshotCreate.Generation,
		RestoreRKEConfig: "",
	}
	err = etcdsnapshot.RestoreSnapshot(client, clusterName, snapshotRestore)
	require.NoError(t, err)

	err = clusters.WaitClusterToBeUpgraded(client, clusterID)
	require.NoError(t, err)

	podResults, podErrors = pods.StatusPods(client, clusterID)
	assert.NotEmpty(t, podResults)
	assert.Empty(t, podErrors)
	require.NoError(t, err)
	require.Equal(t, initialKubernetesVersion, clusterObject.Spec.KubernetesVersion)

	// validate restored workload
	deploymentList, err := steveclient.SteveType(workloads.DeploymentSteveType).NamespacedSteveClient(defaultNamespace).List(nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(deploymentList.Data))
	require.Equal(t, initialWorkloadName, deploymentList.Data[0].ObjectMeta.Name)

	if upgrade != "" {
		clusterObject, _, err := clusters.GetProvisioningClusterByName(adminClient, clusterName, namespace)
		require.NoError(t, err)
		require.Equal(t, initialKubernetesVersion, clusterObject.Spec.KubernetesVersion)
		if strategy {
			require.Equal(t, concurrencyDefaultValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
			require.Equal(t, concurrencyDefaultValue, clusterObject.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
		}
	}
}

func MatchNodeToAnyEtcdRole(t *testing.T, client *rancher.Client, clusterID string) (int, *management.Node) {
	machines, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": clusterID,
	}})
	require.NoError(t, err)
	numOfNodes := 0
	lastMatchingNode := &management.Node{}

	for _, machine := range machines.Data {
		if machine.Etcd {
			lastMatchingNode = &machine
			numOfNodes++
		}
	}
	require.NotEmpty(t, lastMatchingNode.NodeName, "matching node name is empty")
	return numOfNodes, lastMatchingNode
}

func createIngress(client *v1.Client, ingressName string, serviceName string) (*v1.SteveAPIObject, error) {
	podClient := client.SteveType("pod")
	err := kwait.Poll(15*time.Second, 5*time.Minute, func() (done bool, err error) {
		pods, err := podClient.List(nil)
		if err != nil {
			return false, nil
		}
		if len(pods.Data) != 0 {
			return true, nil
		}
		for _, pod := range pods.Data {
			if strings.Contains(pod.Name, "rke2-ingress-nginx") || strings.Contains(pod.Name, "rancher-webhook") {
				_, podError, err := podV1.CheckPodStatus(&pod)
				if err != nil {
					return false, err
				}
				if podError != nil {
					return false, nil
				}
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	path := ingresses.NewIngressPathTemplate(networkingv1.PathTypeExact, ingressPath, serviceName, 80)
	ingressTemplate := ingresses.NewIngressTemplate(ingressName, defaultNamespace, "", []networkingv1.HTTPIngressPath{path})

	logrus.Infof("Creating ingress %v", ingressTemplate)
	logrus.Infof("ingress Name %v", ingressName)
	ingressResp, err := client.SteveType(ingresses.IngressSteveType).Create(ingressTemplate)
	if err != nil {
		return nil, err
	}

	return ingressResp, err
}

func createDeployment(steveclient *steveV1.Client, wlName string) (*steveV1.SteveAPIObject, error) {
	containerTemplate := workloads.NewContainer(containerName, containerImage, corev1.PullAlways, []corev1.VolumeMount{}, []corev1.EnvFromSource{})
	podTemplate := workloads.NewPodTemplate([]corev1.Container{containerTemplate}, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil)
	deployment := workloads.NewDeploymentTemplate(wlName, defaultNamespace, podTemplate, isCattleLabeled, nil)

	deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAppendName + wlName,
			Namespace: defaultNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: "port",
					Port: 80,
				},
			},
			Selector: deployment.Spec.Template.Labels,
		},
	}
	_, err = steveclient.SteveType("service").Create(service)
	if err != nil {
		return nil, err
	}

	return deploymentResp, err
}
