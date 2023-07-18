package rke2

import (
	"fmt"
	"strings"
	"testing"
	"time"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/etcdsnapshot"
	"github.com/rancher/rancher/tests/framework/extensions/ingresses"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/extensions/pipeline"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"

	"github.com/rancher/rancher/tests/framework/pkg/environmentflag"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"

	networkingv1 "k8s.io/api/networking/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
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
	advancedOptions    provisioning.AdvancedOptions
	etcdSnapshotS3     *rkev1.ETCDSnapshotS3
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.ns = defaultNamespace

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	r.kubernetesVersions = clustersConfig.RKE2KubernetesVersions
	r.cnis = clustersConfig.CNIs
	r.providers = clustersConfig.Providers
	r.nodesAndRoles = clustersConfig.NodesAndRoles
	r.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	if clustersConfig.S3BackupConfig != nil {
		r.etcdSnapshotS3 = &rkev1.ETCDSnapshotS3{
			Endpoint:      clustersConfig.S3BackupConfig.Endpoint,
			Bucket:        clustersConfig.S3BackupConfig.BucketName,
			Region:        clustersConfig.S3BackupConfig.Region,
			Folder:        clustersConfig.S3BackupConfig.Folder,
			SkipSSLVerify: true,
		}
		provider := CreateProvider(provisioning.AWSProviderName.String())
		creds, err := provider.CloudCredFunc(client)
		require.NoError(r.T(), err)
		r.etcdSnapshotS3.CloudCredentialName = creds.ID
		logrus.Infof("%v", creds.ID)
	}

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

	clusterName := namegen.AppendRandomString(provider.Name.String())

	logrus.Infof("creating rke2Cluster.............")
	clusterResp, err := createRKE2NodeDriverCluster(client, provider, clusterName, initialK8sVersion, r.ns, r.cnis[0], r.advancedOptions, r.etcdSnapshotS3)
	require.NoError(r.T(), err)
	require.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)
	logrus.Infof("rke2Cluster create request successful.............")

	if r.client.Flags.GetValue(environmentflag.UpdateClusterName) {
		pipeline.UpdateConfigClusterName(clusterName)
	}

	logrus.Infof("creating watch over cluster.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
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

	containerTemplate := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate := workloads.NewPodTemplate([]v1.Container{containerTemplate}, []v1.Volume{}, []v1.LocalObjectReference{}, nil)
	deploymentBeforeBackup := workloads.NewDeploymentTemplate(wloadBeforeRestore, r.ns, podTemplate, isCattleLabeled, nil)

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
	err = etcdsnapshot.CreateSnapshot(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	logrus.Infof("created a snapshot of the cluster.............")

	logrus.Infof("creating watch over cluster after creating a snapshot.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is active again.............")

	var snapshotToBeRestored string

	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		snapshotList, err := getSnapshots(client, localClusterID)
		if err != nil {
			return false, err
		}
		totalClusterSnapShots := 0
		for _, snapshot := range snapshotList {
			prefix := "on-demand-" + clusterName
			if strings.Contains(snapshot.ObjectMeta.Name, prefix) {
				if snapshotToBeRestored == "" {
					snapshotToBeRestored = snapshot.Name
				}
				totalClusterSnapShots++
			}
		}
		if totalClusterSnapShots >= etcdnodeCount {
			return true, nil
		}
		return false, nil
	})
	require.NoError(r.T(), err)

	logrus.Infof("creating watch over pods.............")
	r.watchAndWaitForPods(client, clusterID)
	logrus.Infof("All pods are up and running.............")

	logrus.Infof("creating a workload(w2, deployment).............")
	containerTemplate2 := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate2 := workloads.NewPodTemplate([]v1.Container{containerTemplate2}, []v1.Volume{}, []v1.LocalObjectReference{}, nil)
	deploymentAfterBackup := workloads.NewDeploymentTemplate(wloadAfterBackup, r.ns, podTemplate2, isCattleLabeled, nil)

	deploymentResp, err = createDeployment(deploymentAfterBackup, steveclient, client, clusterID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), deploymentAfterBackup.Name, deploymentResp.ObjectMeta.Name)
	logrus.Infof("%v is ready.............", deploymentAfterBackup.Name)

	logrus.Infof("upgrading cluster k8s version.............")
	k8sUpgradedVersion := r.kubernetesVersions[1]
	err = upgradeClusterK8sVersion(client, clusterName, k8sUpgradedVersion, r.ns)
	require.NoError(r.T(), err)
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is active again.............")

	cluster, _, err := clusters.GetProvisioningClusterByName(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	require.Equal(r.T(), k8sUpgradedVersion, cluster.Spec.KubernetesVersion)

	logrus.Infof("restoring snapshot.............")
	require.NoError(r.T(), restoreSnapshot(client, clusterName, snapshotToBeRestored, 1, "all", r.ns))
	logrus.Infof("successfully submitted restoration request.............")

	logrus.Infof("creating watch over cluster after restore.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
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

	cluster, _, err = clusters.GetProvisioningClusterByName(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	require.Equal(r.T(), initialK8sVersion, cluster.Spec.KubernetesVersion)
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) EtcdSnapshotRestoreWithUpgradeStrategy(provider *Provider) {
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

	clusterName := namegen.AppendRandomString(provider.Name.String())

	logrus.Infof("creating rke2Cluster.............")
	clusterResp, err := createRKE2NodeDriverCluster(client, provider, clusterName, initialK8sVersion, r.ns, r.cnis[0], r.advancedOptions, r.etcdSnapshotS3)
	require.NoError(r.T(), err)
	require.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)
	logrus.Infof("rke2Cluster create request successful.............")

	logrus.Infof("creating watch over cluster.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is up and running.............")

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

	logrus.Infof("creating a workload(nginx deployment).............")

	containerTemplate := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate := workloads.NewPodTemplate([]v1.Container{containerTemplate}, []v1.Volume{}, []v1.LocalObjectReference{}, nil)
	deploymentBeforeBackup := workloads.NewDeploymentTemplate(wloadBeforeRestore, r.ns, podTemplate, isCattleLabeled, nil)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(r.T(), err)

	deploymentResp, err := createDeployment(deploymentBeforeBackup, steveclient, client, clusterID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), deploymentBeforeBackup.Name, deploymentResp.ObjectMeta.Name)
	logrus.Infof("%v is ready.............", deploymentBeforeBackup.Name)

	logrus.Infof("creating an ingress.............")

	path := ingresses.NewIngressPathTemplate(networkingv1.PathTypeExact, "/index.html", wloadServiceName, 80)
	ingressBeforeBackup := ingresses.NewIngressTemplate(ingressName, r.ns, "", []networkingv1.HTTPIngressPath{path})

	ingressResp, err := steveclient.SteveType(ingresses.IngressSteveType).Create(ingressBeforeBackup)
	require.NoError(r.T(), err)

	require.Equal(r.T(), ingressName, ingressResp.ObjectMeta.Name)
	logrus.Infof("created an ingress.............")

	logrus.Infof("creating a snapshot of the cluster.............")
	err = etcdsnapshot.CreateSnapshot(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	logrus.Infof("created a snapshot of the cluster.............")

	logrus.Infof("creating watch over cluster after creating a snapshot.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is active again.............")

	var snapshotToBeRestored string

	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		snapshotList, err := getSnapshots(client, localClusterID)
		if err != nil {
			return false, err
		}
		totalClusterSnapShots := 0
		for _, snapshot := range snapshotList {
			prefix := "on-demand-" + clusterName
			if strings.Contains(snapshot.ObjectMeta.Name, prefix) {
				if snapshotToBeRestored == "" {
					snapshotToBeRestored = snapshot.Name
				}
				totalClusterSnapShots++
			}
		}
		if totalClusterSnapShots >= etcdnodeCount {
			return true, nil
		}
		return false, nil
	})
	require.NoError(r.T(), err)

	logrus.Infof("creating watch over pods.............")
	r.watchAndWaitForPods(client, clusterID)
	logrus.Infof("All pods are up and running.............")

	logrus.Infof("creating a workload(w2, deployment).............")
	containerTemplate2 := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate2 := workloads.NewPodTemplate([]v1.Container{containerTemplate2}, []v1.Volume{}, []v1.LocalObjectReference{}, nil)
	deploymentAfterBackup := workloads.NewDeploymentTemplate(wloadAfterBackup, r.ns, podTemplate2, isCattleLabeled, nil)

	deploymentResp, err = createDeployment(deploymentAfterBackup, steveclient, client, clusterID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), deploymentAfterBackup.Name, deploymentResp.ObjectMeta.Name)
	logrus.Infof("%v is ready.............", deploymentAfterBackup.Name)

	logrus.Infof("upgrading cluster k8s version.............")
	k8sUpgradedVersion := r.kubernetesVersions[1]
	err = upgradeClusterK8sVersionWithUpgradeStrategy(client, clusterName, k8sUpgradedVersion, r.ns)
	require.NoError(r.T(), err)
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is active again.............")

	cluster, _, err := clusters.GetProvisioningClusterByName(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	require.Equal(r.T(), k8sUpgradedVersion, cluster.Spec.KubernetesVersion)

	logrus.Infof("validating ControlPlaneConcurrency and WorkerConcurrency values are updated..")
	require.Equal(r.T(), "15%", cluster.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
	require.Equal(r.T(), "20%", cluster.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)

	logrus.Infof("restoring snapshot.............")
	require.NoError(r.T(), restoreSnapshot(client, clusterName, snapshotToBeRestored, 1, "all", r.ns))
	logrus.Infof("successfully submitted restoration request.............")

	logrus.Infof("creating watch over cluster after restore.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
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

	cluster, _, err = clusters.GetProvisioningClusterByName(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	require.Equal(r.T(), initialK8sVersion, cluster.Spec.KubernetesVersion)
	logrus.Infof("validating ControlPlaneConcurrency and WorkerConcurrency are restored to default values..")
	require.Equal(r.T(), "10%", cluster.Spec.RKEConfig.UpgradeStrategy.ControlPlaneConcurrency)
	require.Equal(r.T(), "10%", cluster.Spec.RKEConfig.UpgradeStrategy.WorkerConcurrency)
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
		var podErrors []error

		for _, pod := range pods.Data {
			if pod.Namespace == cattleSystem && strings.HasPrefix(pod.Name, podPrefix) {
				continue
			}
			podStatus := &v1.PodStatus{}
			err = steveV1.ConvertToK8sType(pod.Status, podStatus)
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
			conditions := podStatus.Conditions
			if phase == v1.PodPending || phase == v1.PodRunning {
				podReady := false
				for _, condition := range conditions {
					if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
						podReady = true
						break
					}
				}
				if !podReady {
					for _, cs := range podStatus.ContainerStatuses {
						if !cs.Ready {
							logrus.Infof("container %s of pod %s is not ready, state: %+v ", cs.Name, pod.Name, cs.State)
						}
					}
					return false, nil
				}

			} else if phase == v1.PodFailed || phase == v1.PodUnknown {
				podErrors = append(podErrors, fmt.Errorf("ERROR: %s: %s", pod.Name, podStatus))
			}
		}
		if len(podErrors) > 0 {
			return false, fmt.Errorf("Error in running pods : %v", podErrors)
		}
		if isIngressControllerPodPresent && isKubeControllerManagerPresent {
			return true, nil
		}
		return false, nil
	})
	require.NoError(r.T(), err)
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) EtcdSnapshotRestore(provider *Provider) {
	initialK8sVersion := r.kubernetesVersions[0]
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	logrus.Infof("creating kube provisioning client.............")
	kubeProvisioningClient, err := r.client.GetKubeAPIProvisioningClient()
	require.NoError(r.T(), err)
	logrus.Infof("kube provisioning client created.............")

	clusterName := namegen.AppendRandomString(provider.Name.String())

	logrus.Infof("creating rke2Cluster.............")
	clusterResp, err := createRKE2NodeDriverCluster(client, provider, clusterName, initialK8sVersion, r.ns, r.cnis[0], r.advancedOptions, r.etcdSnapshotS3)
	require.NoError(r.T(), err)
	require.Equal(r.T(), clusterName, clusterResp.ObjectMeta.Name)
	logrus.Infof("rke2Cluster create request successful.............")

	logrus.Infof("creating watch over cluster.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is up and running.............")

	logrus.Info("getting cluster id.............")
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	require.NoError(r.T(), err)
	logrus.Info("got cluster id.............", clusterID)

	logrus.Info("getting local cluster id.............")
	localClusterID, err := clusters.GetClusterIDByName(client, localClusterName)
	require.NoError(r.T(), err)
	logrus.Info("got local cluster id.............", localClusterID)

	logrus.Infof("creating watch over pods.............")
	err = watchAndWaitForPods(client, clusterID)
	require.NoError(r.T(), err)
	logrus.Infof("All pods are up and running.............")

	// creating the workload W1
	logrus.Infof("creating a workload(nginx deployment).............")

	containerTemplate := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate := workloads.NewPodTemplate([]v1.Container{containerTemplate}, []v1.Volume{}, []v1.LocalObjectReference{}, nil)
	deploymentBeforeBackup := workloads.NewDeploymentTemplate(wloadBeforeRestore, r.ns, podTemplate, isCattleLabeled, nil)

	// creating steve client
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	require.NoError(r.T(), err)

	deploymentResp, err := createDeployment(deploymentBeforeBackup, steveclient, client, clusterID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), deploymentBeforeBackup.Name, deploymentResp.ObjectMeta.Name)
	logrus.Infof("%v is ready.............", deploymentBeforeBackup.Name)

	// creating the ingress1
	logrus.Infof("creating an ingress.............")

	path := ingresses.NewIngressPathTemplate(networkingv1.PathTypeExact, "/index.html", wloadServiceName, 80)
	ingressBeforeBackup := ingresses.NewIngressTemplate(ingressName, r.ns, "", []networkingv1.HTTPIngressPath{path})

	ingressResp, err := steveclient.SteveType(ingresses.IngressSteveType).Create(ingressBeforeBackup)
	require.NoError(r.T(), err)

	require.Equal(r.T(), ingressName, ingressResp.ObjectMeta.Name)
	logrus.Infof("created an ingress.............")

	logrus.Infof("creating a snapshot of the cluster.............")
	err = etcdsnapshot.CreateSnapshot(client, clusterName, r.ns)
	require.NoError(r.T(), err)
	logrus.Infof("created a snapshot of the cluster.............")

	logrus.Infof("creating watch over cluster after creating a snapshot.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is active again.............")

	logrus.Infof("creating watch over pods.............")
	r.watchAndWaitForPods(client, clusterID)
	logrus.Infof("All pods are up and running.............")
	var snapshotToBeRestored string

	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		snapshotList, err := getSnapshots(client, localClusterID)
		if err != nil {
			return false, err
		}
		totalClusterSnapShots := 0
		for _, snapshot := range snapshotList {
			if strings.Contains(snapshot.ObjectMeta.Name, s3BackupPrefix+clusterName) {
				if snapshotToBeRestored == "" {
					snapshotToBeRestored = snapshot.Name
				}
				totalClusterSnapShots++
			}
		}
		if totalClusterSnapShots >= etcdnodeCount {
			return true, nil
		}
		return false, nil
	})
	require.NoError(r.T(), err)

	logrus.Infof("creating watch over pods.............")
	r.watchAndWaitForPods(client, clusterID)
	logrus.Infof("All pods are up and running.............")

	logrus.Infof("creating a workload(w2, deployment).............")
	containerTemplate2 := workloads.NewContainer("ngnix", "nginx", v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{})
	podTemplate2 := workloads.NewPodTemplate([]v1.Container{containerTemplate2}, []v1.Volume{}, []v1.LocalObjectReference{}, nil)
	deploymentAfterBackup := workloads.NewDeploymentTemplate(wloadAfterBackup, r.ns, podTemplate2, isCattleLabeled, nil)

	deploymentResp, err = createDeployment(deploymentAfterBackup, steveclient, client, clusterID)
	require.NoError(r.T(), err)
	require.Equal(r.T(), deploymentAfterBackup.Name, deploymentResp.ObjectMeta.Name)
	logrus.Infof("%v is ready.............", deploymentAfterBackup.Name)

	logrus.Infof("restoring snapshot.............")
	require.NoError(r.T(), restoreSnapshot(client, clusterName, snapshotToBeRestored, 1, "", r.ns))
	logrus.Infof("successfully submitted restoration request.............")

	logrus.Infof("creating watch over cluster after restore.............")
	clusters.WatchAndWaitForCluster(r.client.Steve, kubeProvisioningClient, r.ns, clusterName)
	logrus.Infof("cluster is active again.............")

	logrus.Infof("creating watch over pods.............")
	err = watchAndWaitForPods(client, clusterID)
	require.NoError(r.T(), err)
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

}

func TestEtcdSnapshotRestore(t *testing.T) {
	suite.Run(t, new(RKE2EtcdSnapshotRestoreTestSuite))
}

func (r *RKE2EtcdSnapshotRestoreTestSuite) TestEtcdOnlySnapshotRestore() {
	logrus.Infof("checking for valid k8s versions and cnis in the configuration....")
	require.GreaterOrEqualf(r.T(), len(r.kubernetesVersions), 1, "At least one k8s version is required in the config")
	require.GreaterOrEqualf(r.T(), len(r.cnis), 1, "At least one cni is required in the config")
	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.EtcdSnapshotRestore(&provider)
	}
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

func (r *RKE2EtcdSnapshotRestoreTestSuite) TestEtcdSnapshotRestoreWithUpgradeStrategy() {
	logrus.Infof("checking for valid k8s versions and cnis in the configuration....")
	require.GreaterOrEqualf(r.T(), len(r.kubernetesVersions), 2, "Two k8s versions are required in the config")
	require.GreaterOrEqualf(r.T(), len(r.cnis), 1, "At least one cni is required in the config")
	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.EtcdSnapshotRestoreWithUpgradeStrategy(&provider)
	}
}
