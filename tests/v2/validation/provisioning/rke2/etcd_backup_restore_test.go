package rke2

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	provisioningV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/ingresses"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"

	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeProvisioning "github.com/rancher/rancher/tests/framework/clients/provisioning"
	networkingv1 "k8s.io/api/networking/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	defaultNamespace = "default"
)

type RKE2EtcdSnapshotRestoreTestSuite struct {
	suite.Suite
	session            *session.Session
	client             *rancher.Client
	ns                 string
	kubernetesVersions []string
	cnis               []string
	providers          []string
	nodesAndRoles      []machinepools.NodeRoles
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) SetupSuite() {
	testSession := session.NewSession(r.T())
	r.session = testSession

	r.ns = defaultNamespace

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.nodesAndRoles = clustersConfig.NodesAndRoles

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

}

func (r *RKE2EtcdSnapshotRestoreTestSuite) EtcdSnapshotRestoreWithK8sUpgrade(provider *Provider) {
	initialK8sVersion := r.kubernetesVersions[0]
	logrus.Infof("running etcd snapshot restore test.............")
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	logrus.Infof("creating kube provisioning client.............")
	kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
	require.NoError(r.T(), err)
	logrus.Infof("kube provisioning client created.............")

	clusterName := namegen.AppendRandomString(provider.Name)

	logrus.Infof("creating rke2Cluster.............")
	clusterResp, err := createRKE2NodeDriverCluster(client, provider, clusterName, initialK8sVersion, r.ns, r.cnis[0])
	require.NoError(r.T(), err)
	require.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)
	logrus.Infof("rke2Cluster create request successful.............")

	logrus.Infof("creating watch over cluster.............")
	r.watchAndWaitForCluster(kubeProvisioningClient, clusterName)
	logrus.Infof("cluster is up and running.............")

	// Get clusterID by clusterName
	logrus.Info("getting cluster id.............")
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(r.T(), err)
	logrus.Info("got cluster id.............", clusterID)

	logrus.Info("getting local cluster id.............")
	localClusterID, err := clusters.GetClusterIDByName(client, localClusterName)
	require.NoError(r.T(), err)
	logrus.Info("got local cluster id.............", localClusterID)

	logrus.Infof("creating watch over pods.............")
	r.watchAndWaitForPods(client, clusterID)
	logrus.Infof("All pods are up and running.............")

	// creating the workload W1
	logrus.Infof("creating a workload(nginx deployment).............")

	wloadBeforeRestoreLabels := map[string]string{}
	wloadBeforeRestoreLabels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", r.ns, wloadBeforeRestore)

	containerTemplate := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate := workloads.NewPodTemplate([]v1.Container{containerTemplate}, []v1.Volume{}, []v1.LocalObjectReference{}, wloadBeforeRestoreLabels)
	deploymentBeforeBackup := workloads.NewDeploymentTemplate(wloadBeforeRestore, r.ns, podTemplate, wloadBeforeRestoreLabels)

	// creating steve client
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(r.T(), err)

	deploymentResp, err := createDeployment(deploymentBeforeBackup, steveclient, client, clusterID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), deploymentBeforeBackup.Name, deploymentResp.ObjectMeta.Name)
	logrus.Infof("%v is ready.............", deploymentBeforeBackup.Name)

	// creating the ingress1
	logrus.Infof("creating an ingress.............")

	exactPath := networkingv1.PathTypeExact
	paths := []networkingv1.HTTPIngressPath{
		{
			Path:     "/index.html",
			PathType: &exactPath,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: wloadServiceName,
					Port: networkingv1.ServiceBackendPort{
						Number: 80,
					},
				},
			},
		},
	}
	ingressBeforeBackup := ingresses.NewIngressTemplate(ingressName, r.ns, "", paths)

	ingressResp, err := steveclient.SteveType(ingresses.IngressSteveType).Create(ingressBeforeBackup)
	require.NoError(r.T(), err)

	require.Equal(r.T(), ingressName, ingressResp.ObjectMeta.Name)
	logrus.Infof("created an ingress.............")

	logrus.Infof("creating a snapshot of the cluster.............")
	err = createSnapshot(client, clusterName, 1, r.ns)
	require.NoError(r.T(), err)
	logrus.Infof("created a snapshot of the cluster.............")

	logrus.Infof("creating watch over cluster after creating a snapshot.............")
	r.watchAndWaitForCluster(kubeProvisioningClient, clusterName)
	logrus.Infof("cluster is active again.............")

	var snapshotToBeRestored string

	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		snapshotList, err := getSnapshots(client, localClusterID)
		if err != nil {
			return false, err
		}
		totalClusterSnapShots := 0
		for _, snapshot := range snapshotList {
			if strings.Contains(snapshot.ObjectMeta.Name, clusterName) {
				if snapshotToBeRestored == "" {
					snapshotToBeRestored = snapshot.Name
				}
				totalClusterSnapShots++
			}
		}
		if totalClusterSnapShots == etcdnodeCount {
			return true, nil
		}
		return false, nil
	})
	require.NoError(r.T(), err)

	logrus.Infof("creating a workload(w2, deployment).............")
	wloadAfterBackupLabels := map[string]string{}
	wloadAfterBackupLabels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", r.ns, wloadAfterRestore)
	containerTemplate2 := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate2 := workloads.NewPodTemplate([]v1.Container{containerTemplate2}, []v1.Volume{}, []v1.LocalObjectReference{}, wloadAfterBackupLabels)
	deploymentAfterBackup := workloads.NewDeploymentTemplate(wloadAfterRestore, r.ns, podTemplate2, wloadAfterBackupLabels)

	deploymentResp, err = createDeployment(deploymentAfterBackup, steveclient, client, clusterID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), deploymentAfterBackup.Name, deploymentResp.ObjectMeta.Name)
	logrus.Infof("%v is ready.............", deploymentAfterBackup.Name)

	logrus.Infof("upgrading cluster k8s version.............")
	k8sUpgradedVersion := r.kubernetesVersions[1]
	err = upgradeClusterK8sVersion(client, clusterName, k8sUpgradedVersion, r.ns)
	require.NoError(r.T(), err)
	r.watchAndWaitForCluster(kubeProvisioningClient, clusterName)
	logrus.Infof("cluster is active again.............")

	cluster, _, err := getProvisioningClusterByName(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	require.Equal(r.T(), k8sUpgradedVersion, cluster.Spec.KubernetesVersion)

	logrus.Infof("restoring snapshot.............")
	require.NoError(r.T(), restoreSnapshot(client, clusterName, snapshotToBeRestored, 1, "all", r.ns))
	logrus.Infof("successfully submitted restoration request.............")

	logrus.Infof("creating watch over cluster after restore.............")
	r.watchAndWaitForCluster(kubeProvisioningClient, clusterName)
	logrus.Infof("cluster is active again.............")

	logrus.Infof("creating watch over pods.............")
	r.watchAndWaitForPods(client, clusterID)
	logrus.Infof("All pods are up and running.............")

	logrus.Infof("fetching deployment list to validate restore.............")
	deploymentList, err := steveclient.SteveType(workloads.DeploymentSteveType).NamespacedSteveClient(r.ns).List(nil)
	require.NoError(r.T(), err)
	require.Equal(r.T(), 1, len(deploymentList.Data))
	require.Equal(r.T(), wloadBeforeRestore, deploymentList.Data[0].ObjectMeta.Name)
	logrus.Infof(" deployment list validated successfully.............")

	logrus.Infof("fetching ingresses list to validate restore.............")
	ingressResp, err = steveclient.SteveType(ingresses.IngressSteveType).ByID(r.ns + "/" + ingressBeforeBackup.Name)
	require.NoError(r.T(), err)
	require.NotNil(r.T(), ingressResp)
	logrus.Infof("ingress validated successfully.............")

	cluster, _, err = getProvisioningClusterByName(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	require.Equal(r.T(), initialK8sVersion, cluster.Spec.KubernetesVersion)
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) watchAndWaitForCluster(kubeProvisioningClient *kubeProvisioning.Client, clusterName string) {
	err := kwait.Poll(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		clusterResp, err := r.client.Steve.SteveType(ProvisioningSteveResouceType).ByID(r.ns + "/" + clusterName)
		if err != nil {
			return false, err
		}
		state := clusterResp.ObjectMeta.State.Name
		return state != "active", nil
	})
	require.NoError(r.T(), err)
	logrus.Infof("waiting for cluster to be up.............")
	result, err := kubeProvisioningClient.Clusters(r.ns).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector:  "metadata.name=" + clusterName,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(r.T(), err)

	checkFunc := clusters.IsProvisioningClusterReady
	err = wait.WatchWait(result, checkFunc)
	require.NoError(r.T(), err)
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) watchAndWaitForPods(client *rancher.Client, clusterID string) {
	logrus.Infof("waiting for all Pods to be up.............")
	err := kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		steveClient, err := client.Steve.ProxyDownstream(clusterID)
		if err != nil {
			return false, nil
		}
		pods, err := steveClient.SteveType(pods.PodResourceSteveType).List(nil)
		if err != nil {
			return false, nil
		}
		isIngressControllerPodPresent := false
		isKubeControllerManagerPresent := false
		for _, pod := range pods.Data {
			podStatus := &v1.PodStatus{}
			err = provisioningV1.ConvertToK8sType(pod.Status, podStatus)
			if err != nil {
				return false, err
			}
			if !isIngressControllerPodPresent && strings.Contains(pod.ObjectMeta.Name, "ingress-nginx-controller") {
				isIngressControllerPodPresent = true
			}
			if !isKubeControllerManagerPresent && strings.Contains(pod.ObjectMeta.Name, "kube-controller-manager") {
				isKubeControllerManagerPresent = true
			}

			phase := podStatus.Phase
			if phase != v1.PodRunning && phase != v1.PodSucceeded {
				return false, nil
			}

		}
		if isIngressControllerPodPresent && isKubeControllerManagerPresent {
			return true, nil
		}
		return false, nil
	})
	require.NoError(r.T(), err)
}
func TestEtcdSnapshotRestore(t *testing.T) {
	suite.Run(t, new(RKE2EtcdSnapshotRestoreTestSuite))
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) TestEtcdSnapshotRestoreWithK8sUpgrade() {
	logrus.Infof("checking for valid k8s versions and cnis in the configuration....")
	require.GreaterOrEqual(r.T(), len(r.kubernetesVersions), 2)
	require.GreaterOrEqual(r.T(), len(r.cnis), 1)
	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.EtcdSnapshotRestoreWithK8sUpgrade(&provider)
	}
}
