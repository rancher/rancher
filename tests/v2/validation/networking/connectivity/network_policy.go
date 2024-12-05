package connectivity

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"slices"
	"strconv"
	"strings"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	kubeapinodes "github.com/rancher/shepherd/extensions/kubeapi/nodes"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/rancher/shepherd/extensions/workloads"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	pingPodProjectName     = "ping-project"
	containerName          = "test1"
	containerImage         = "ranchertest/mytestcontainer"
	labelWorker            = "labelSelector=node-role.kubernetes.io/worker=true"
	kubeSystemNamespace    = "kube-system"
	cloudControllerManager = "aws-cloud-controller-manager"
)

type resourceNames struct {
	core   map[string]string
	random map[string]string
}

// newNames returns a new resourceNames struct
// it creates a random names with random suffix for each resource by using core and coreWithSuffix names
func newNames() *resourceNames {
	const (
		projectName             = "upgrade-wl-project"
		namespaceName           = "namespace"
		deploymentName          = "deployment"
		daemonsetName           = "daemonset"
		secretName              = "secret"
		serviceName             = "service"
		ingressName             = "ingress"
		defaultRandStringLength = 3
	)

	names := &resourceNames{
		core: map[string]string{
			"projectName":    projectName,
			"namespaceName":  namespaceName,
			"deploymentName": deploymentName,
			"daemonsetName":  daemonsetName,
			"secretName":     secretName,
			"serviceName":    serviceName,
			"ingressName":    ingressName,
		},
	}

	names.random = map[string]string{}
	for k, v := range names.core {
		names.random[k] = v + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	}

	return names
}

// newPodTemplateWithTestContainer is a private constructor that returns pod template spec for workload creations
func newPodTemplateWithTestContainer() corev1.PodTemplateSpec {
	testContainer := newTestContainerMinimal()
	containers := []corev1.Container{testContainer}
	return workloads.NewPodTemplate(containers, nil, []corev1.LocalObjectReference{}, nil, nil)
}

// newTestContainerMinimal is a private constructor that returns container for minimal workload creations
func newTestContainerMinimal() corev1.Container {
	pullPolicy := corev1.PullAlways
	return workloads.NewContainer(containerName, containerImage, pullPolicy, nil, nil, nil, nil, nil)
}

// curlCommand is a helper to run a curl command on an SSH shell node
func curlCommand(client *rancher.Client, clusterID string, url string) (string, error) {
	logrus.Infof("Executing the kubectl command curl %s on the node", url)
	execCmd := []string{"curl", url}
	log, err := kubectl.Command(client, nil, clusterID, execCmd, "")
	if err != nil {
		return "", err
	}
	return log, nil
}

// This must be a valid port number, 10250 < hostPort < 65536
// The range 1-10250 are reserved to Rancher
// Using a random port to avoid 'port in use' failures and allow the test to be rerun
func getHostPort() int {
	return rand.IntN(55283) + 10251
}

// It will allocate a port from a range 30000-32767
// Using a random port to avoid 'port in use' failures and allow the test to be rerun
func getNodePort() int {
	return rand.IntN(2767) + 30000
}

func isCloudManagerEnabled(client *rancher.Client, clusterID string) (bool, error) {
	logrus.Info("Checking cluster version and if the cloud-controller-manager is installed")
	catalogClient, err := client.GetClusterCatalogClient(clusterID)
	if err != nil {
		return false, err
	}

	provisioningClusterID, err := clusters.GetV1ProvisioningClusterByName(client, client.RancherConfig.ClusterName)
	if err != nil {
		return false, err
	}

	cluster, err := client.Steve.SteveType(clusters.ProvisioningSteveResourceType).ByID(provisioningClusterID)
	if err != nil {
		return false, err
	}

	newCluster := &provv1.Cluster{}
	err = steveV1.ConvertToK8sType(cluster, newCluster)
	if err != nil {
		return false, err
	}

	if strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") {
		wranglerContext, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return false, err
		}

		latestDaemonset, err := wranglerContext.Apps.DaemonSet().List("kube-system", metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		for _, item := range latestDaemonset.Items {
			if strings.Contains(item.Name, "svclb-traefik") {
				return true, nil
			}
		}

		return false, nil
	}

	_, err = catalogClient.Apps(kubeSystemNamespace).Get(context.TODO(), cloudControllerManager, metav1.GetOptions{})
	if !strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") && err != nil && strings.Contains(err.Error(), "not found") {
		return false, nil
	}

	return true, nil
}

func isNodePool(steveClient *steveV1.Client) (bool, error) {
	logrus.Info("Checking node pool")

	logrus.Infof("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	if err != nil {
		return false, err
	}

	nodeList, err := steveClient.SteveType("node").List(query)
	if err != nil {
		return false, err
	}

	return len(nodeList.Data) >= 3, err
}

func validateLoadBalancer(client *rancher.Client, clusterID string, steveClient *steveV1.Client, nodePort int, workloadName string) (bool, error) {
	logrus.Infof("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	if err != nil {
		return false, err
	}

	nodeList, err := steveClient.SteveType("node").List(query)
	if err != nil {
		return false, err
	}

	for _, machine := range nodeList.Data {
		logrus.Info("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		if err != nil {
			return false, err
		}

		nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeExternalIP)
		if nodeIP == "" {
			nodeIP = kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)
		}

		log, err := curlCommand(client, clusterID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(nodePort)))
		if err != nil {
			return false, err
		}

		return strings.Contains(log, workloadName), err
	}

	return false, err
}

func validateHostPortSSH(client *rancher.Client, clusterID string, clusterName string, steveClient *steveV1.Client, hostPort int, workloadName string, namespaceName string) (bool, error) {
	logrus.Infof("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	if err != nil {
		return false, err
	}

	nodeList, err := steveClient.SteveType("node").List(query)
	if err != nil {
		return false, err
	}

	_, stevecluster, err := clusters.GetProvisioningClusterByName(client, clusterName, provisioninginput.Namespace)
	if err != nil {
		return false, err
	}

	wc, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	if err != nil {
		return false, err
	}

	pods, err := wc.Core.Pod().List(namespaceName, metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	var nodes []string
	nodes = make([]string, 0)
	for i := 0; i < len(pods.Items); i++ {
		nodeName := pods.Items[i].Spec.NodeName
		nodes = append(nodes, nodeName)
	}

	for _, machine := range nodeList.Data {
		logrus.Info("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		if err != nil {
			return false, err
		}

		_, found := slices.BinarySearch(nodes, newNode.Name)
		if found {
			nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)

			sshUser, err := sshkeys.GetSSHUser(client, stevecluster)
			if err != nil {
				return false, err
			}

			sshNode, err := sshkeys.GetSSHNodeFromMachine(client, sshUser, &machine)
			if err != nil {
				return false, err
			}

			log, err := sshNode.ExecuteCommand(fmt.Sprintf("curl %s:%s/name.html", nodeIP, strconv.Itoa(hostPort)))
			if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
				return false, err
			}

			logrus.Infof("Log of the curl command {%v}", log)
			return strings.Contains(log, workloadName), err
		}
	}

	return false, err
}

func validateHostPort(client *rancher.Client, clusterID string, steveClient *steveV1.Client, hostPort int, workloadName string) (bool, error) {
	logrus.Infof("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	if err != nil {
		return false, err
	}

	nodeList, err := steveClient.SteveType("node").List(query)
	if err != nil {
		return false, err
	}

	for _, machine := range nodeList.Data {
		logrus.Info("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)
		if err != nil {
			return false, err
		}

		nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)

		log, err := curlCommand(client, clusterID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(hostPort)))
		if err != nil {
			return false, err
		}

		return strings.Contains(log, workloadName), err
	}

	return false, err
}

func validateNodePort(client *rancher.Client, clusterID string, steveClient *steveV1.Client, nodePort int, workloadName string) (bool, error) {
	logrus.Infof("Getting the node using the label [%v]", labelWorker)
	query, err := url.ParseQuery(labelWorker)
	if err != nil {
		return false, err
	}

	nodeList, err := steveClient.SteveType("node").List(query)
	if err != nil {
		return false, err
	}

	for _, machine := range nodeList.Data {
		logrus.Info("Getting the node IP")
		newNode := &corev1.Node{}
		err = steveV1.ConvertToK8sType(machine.JSONResp, newNode)

		nodeIP := kubeapinodes.GetNodeIP(newNode, corev1.NodeExternalIP)
		if nodeIP == "" {
			nodeIP = kubeapinodes.GetNodeIP(newNode, corev1.NodeInternalIP)
		}

		log, err := curlCommand(client, clusterID, fmt.Sprintf("%s:%s/name.html", nodeIP, strconv.Itoa(nodePort)))
		if err != nil {
			return false, err
		}

		return strings.Contains(log, workloadName), err
	}

	return false, err
}

func validateClusterIP(client *rancher.Client, clusterID string, steveClient *steveV1.Client, serviceID string, hostPort int, workloadName string) (bool, error) {
	serviceResp, err := steveClient.SteveType(services.ServiceSteveType).ByID(serviceID)
	if err != nil {
		return false, err
	}

	logrus.Info("Getting the cluster IP")
	newService := &corev1.Service{}
	err = steveV1.ConvertToK8sType(serviceResp.JSONResp, newService)
	if err != nil {
		return false, err
	}

	clusterIP := newService.Spec.ClusterIP

	log, err := curlCommand(client, clusterID, fmt.Sprintf("%s:%s/name.html", clusterIP, strconv.Itoa(hostPort)))
	if err != nil {
		return false, err
	}

	return strings.Contains(log, workloadName), err
}

func validateWorkload(client *rancher.Client, clusterID string, deployment *appv1.Deployment, image string, expectedReplicas int, namespaceName string) (bool, error) {
	logrus.Info("Waiting deployment comes up active")
	err := charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deployment.Name,
	})
	if err != nil {
		return false, err
	}

	logrus.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespaceName, deployment)
	if err != nil {
		return false, err
	}

	logrus.Infof("Counting all pods running by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterID, namespaceName, image)
	if err != nil {
		return false, err
	}

	return expectedReplicas == countPods, err
}
