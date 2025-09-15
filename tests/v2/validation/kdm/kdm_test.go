// //go:build validation

package kdm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	stevev1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	rancherDeployment    = "rancher"
	rancherNamespace     = "cattle-system"
	rancherLabelSelector = "app=rancher"
	rkeMetadataConfig    = "rke-metadata-config"
)

var defaultBackoff = wait.Backoff{
	Duration: 1 * time.Second,
	Factor:   1.5,
	Steps:    10,
}

type KDMTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	cluster   *management.Cluster
	config    *rest.Config
	clientset *kubernetes.Clientset
}

func (k *KDMTestSuite) SetupSuite() {
	var err error
	k.session = session.NewSession()
	k.client, err = rancher.NewClient("", k.session)
	k.Require().NoError(err, "Failed to create RancherClient")

	k.cluster, err = k.client.Management.Cluster.ByID("local")
	k.Require().NoError(err)
	k.Require().NotEmpty(k.cluster)

	localClusterKubeconfig, err := k.client.Management.Cluster.ActionGenerateKubeconfig(k.cluster)
	k.Require().NoError(err)

	c, err := clientcmd.NewClientConfigFromBytes([]byte(localClusterKubeconfig.Config))
	k.Require().NoError(err)

	k.config, err = c.ClientConfig()
	k.Require().NoError(err)

	k.clientset, err = kubernetes.NewForConfig(k.config)
	k.Require().NoError(err)
}

func (k *KDMTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *KDMTestSuite) updateKDMurl(value string) {
	existing, err := k.client.Steve.SteveType("management.cattle.io.setting").ByID(rkeMetadataConfig)
	k.Require().NoError(err, "error getting existing setting")

	var kdmSetting v3.Setting
	err = stevev1.ConvertToK8sType(existing.JSONResp, &kdmSetting)
	k.Require().NoError(err, "error converting existing setting")

	kdmData := map[string]string{}
	err = json.Unmarshal([]byte(kdmSetting.Value), &kdmData)
	k.Require().NoError(err, "error unmarshaling existing setting")

	kdmData["url"] = value
	val, err := json.Marshal(kdmData)
	k.Require().NoError(err, "error marshaling existing setting")
	kdmSetting.Value = string(val)
	_, err = k.client.Steve.SteveType("management.cattle.io.setting").Update(existing, kdmSetting)
	k.Require().NoError(err, "error updating setting")
}

func (k *KDMTestSuite) scaleRancherTo(desiredReplicas int32) {
	deployment, err := k.clientset.AppsV1().Deployments(rancherNamespace).Get(context.TODO(), rancherDeployment, metav1.GetOptions{})
	k.Require().NoError(err, "error getting rancher deployment")

	if *deployment.Spec.Replicas == desiredReplicas {
		return
	}
	*deployment.Spec.Replicas = desiredReplicas

	deployment, err = k.clientset.AppsV1().Deployments(rancherNamespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	k.Require().NoError(err, "error updating rancher deployment")

	// Wait for the deployment to scale up using exponential defaultBackoff
	err = wait.ExponentialBackoff(defaultBackoff, func() (bool, error) {
		deployment, err = k.clientset.AppsV1().Deployments(rancherNamespace).Get(context.TODO(), rancherDeployment, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("Error getting deployment: %s", err.Error())
		}

		if deployment.Status.ReadyReplicas == desiredReplicas {
			log.Infof("Deployment %s successfully scaled to %d replicas", rancherDeployment, desiredReplicas)
			return true, nil
		}
		log.Infof("Waiting for deployment %s to scale. Current replicas: %d/%d", rancherDeployment, deployment.Status.ReadyReplicas, desiredReplicas)
		return false, nil
	})
	k.Require().NoError(err, "error scaling rancher deployment, timed out")

	// arbitrary sleep for leader election activity to take place on the newly created replicas
	time.Sleep(15 * time.Second)
}

func (k *KDMTestSuite) getRancherReplicas() *v1.PodList {
	podList, err := k.clientset.CoreV1().Pods(rancherNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: rancherLabelSelector})
	k.Require().NoError(err, "error getting rancher pod list")
	return podList
}

func (k *KDMTestSuite) execCMDForKDMDump(pod v1.Pod, cmd []string) string {
	log.Infof("Exec request for Pod:%v, Command:%v", pod.Name, cmd)
	request := k.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		SetHeader("Upgrade", "websocket").
		SetHeader("Sec-Websocket-Key", "websocket").
		SetHeader("Sec-Websocket-Version", "13").
		SetHeader("Connection", "Upgrade").
		VersionedParams(&v1.PodExecOptions{
			Container: "rancher",
			Command:   cmd,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec).Do(context.TODO())

	result, err := request.Raw()
	k.Require().NoError(err, "error executing command")
	return string(result)
}

func findLatestVersion(allVersions []string) (string, error) {
	var latest *semver.Version
	for _, version := range allVersions {
		ver, err := semver.NewVersion(version)
		if err != nil {
			return "", err
		}
		if latest == nil || ver.GreaterThan(latest) {
			latest = ver
		}
	}
	if latest == nil {
		return "", fmt.Errorf("no valid versions found")
	}
	return latest.String(), nil
}

func (k *KDMTestSuite) TestChangeKDMurl() {
	// set kdm url to a branch NOT containing newer versions
	// NOTE: This does NOT need to be changed when switching KDM branches from `dev` to `release` and vice-versa
	k.updateKDMurl("https://raw.githubusercontent.com/rancher/kontainer-driver-metadata/refs/heads/dev-v2.9-2024-07-patches/data/data.json")

	// scale Rancher to 3 replicas
	k.scaleRancherTo(3)

	// get the current release value
	availableRKE2Versions, err := kubernetesversions.ListRKE2AllVersions(k.client)
	k.Require().NoError(err, "error listing RKE2 versions")
	currentRKE2Version, err := findLatestVersion(availableRKE2Versions)
	k.Require().NoError(err, "error getting kubernetes version")
	log.Infof("current RKE2 version available: %s", currentRKE2Version)

	// change KDM URL to branch containing newer versions
	// NOTE: This does NOT need to be changed when switching KDM branches from `dev` to `release` and vice-versa
	k.updateKDMurl("https://releases.rancher.com/kontainer-driver-metadata/dev-v2.9/data.json")

	var updatedRKE2Version string
	// check latest Release value
	err = wait.ExponentialBackoff(defaultBackoff, func() (bool, error) {
		currentAvailableRKE2Versions, err := kubernetesversions.ListRKE2AllVersions(k.client)
		if err != nil {
			return false, fmt.Errorf("error getting kubernetes version: %s", err.Error())
		}
		updatedRKE2Version, err = findLatestVersion(currentAvailableRKE2Versions)
		if updatedRKE2Version != currentRKE2Version {
			// change detected
			return true, nil
		}
		return false, nil
	})
	log.Infof("New available RKE2 version: %s", updatedRKE2Version)
	if updatedRKE2Version != currentRKE2Version {
		// look for updated version in all Rancher Pod
		cmd := []string{"curl", "--insecure", "https://0.0.0.0/v1-rke2-release/releases"}
		pods := k.getRancherReplicas()
		for _, pod := range pods.Items {
			output := k.execCMDForKDMDump(pod, cmd)
			// if the curl output to from the pod to itslef (over 0.0.0.0) doesn't contain (trace of) version we
			// received over API call which most likely went to the leader Pod, we can assume the KDM file was
			// NOT updated on all Pods
			if !strings.Contains(output, updatedRKE2Version) {
				k.Fail(fmt.Sprintf("found KDM from a pod:%v not having the latest known version:%v", pod.Name, updatedRKE2Version))
			}
		}
	} else {
		// This is the scenario where both release and dev version of KDM have same latest version
		log.Infof("Latest RKE2 versions on both release and dev KDM channels seem to be the same, %v & %v", currentRKE2Version, updatedRKE2Version)
	}
}

func TestKDMTestSuite(t *testing.T) {
	suite.Run(t, new(KDMTestSuite))
}
