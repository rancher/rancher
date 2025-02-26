package services

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionClusters "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/ingresses"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	active              = "active"
	noSuchHostSubString = "no such host"
)

// VerifyService waits for a service to be ready in the downstream cluster
func VerifyService(steveclient *steveV1.Client, serviceResp *steveV1.SteveAPIObject) error {
	err := kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.OneMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		service, err := steveclient.SteveType(ServiceSteveType).ByID(serviceResp.ID)
		if err != nil {
			return false, nil
		}

		if service.State.Name == active {
			logrus.Infof("Successfully created service: %s", service.Name)

			return true, nil
		}

		return false, nil
	})

	return err
}

// VerifyAWSLoadBalancer validates that an AWS loadbalancer service is created and working properly
func VerifyAWSLoadBalancer(t *testing.T, client *rancher.Client, serviceLB *v1.SteveAPIObject, clusterName string) {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	require.NoError(t, err)

	steveclient, err := adminClient.Steve.ProxyDownstream(clusterName)
	require.NoError(t, err)

	lbHostname := ""
	err = kwait.Poll(5*time.Second, 1*time.Minute, func() (done bool, err error) {
		updateService, err := steveclient.SteveType("service").ByID(serviceLB.ID)
		if err != nil {
			return false, nil
		}

		serviceStatus := &corev1.ServiceStatus{}
		err = v1.ConvertToK8sType(updateService.Status, serviceStatus)
		if err != nil {
			return false, err
		}
		if len(serviceStatus.LoadBalancer.Ingress) == 0 {
			return false, nil
		}

		lbHostname = serviceStatus.LoadBalancer.Ingress[0].Hostname
		return true, nil
	})
	require.NoError(t, err)

	err = kwait.Poll(5*time.Second, 3*time.Minute, func() (done bool, err error) {
		isIngressAccessible, err := ingresses.IsIngressExternallyAccessible(client, lbHostname, "", false)
		if err != nil {
			if strings.Contains(err.Error(), noSuchHostSubString) {
				return false, nil
			}
			return false, err
		}

		return isIngressAccessible, nil
	})
	require.NoError(t, err)
}

// VerifyClusterIP is a helper function that verifies the cluster is able to connect to the cluster ip service by ssh shell
func VerifyClusterIP(client *rancher.Client, clusterName string, clusterID string, serviceID string, path string, content string) error {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	serviceResp, err := steveClient.SteveType(ServiceSteveType).ByID(serviceID)
	if err != nil {
		return err
	}

	logrus.Info("Getting the cluster IP")
	newService := &corev1.Service{}
	err = steveV1.ConvertToK8sType(serviceResp.JSONResp, newService)
	if err != nil {
		return err
	}

	clusterIP := newService.Spec.ClusterIP

	logrus.Infof("Getting the node using the label [%v]", clusters.LabelWorker)
	query, err := url.ParseQuery(clusters.LabelWorker)
	if err != nil {
		return err
	}

	nodeList, err := steveClient.SteveType("node").List(query)
	if err != nil {
		return err
	}

	provisioningClusterID, err := extensionClusters.GetV1ProvisioningClusterByName(client, clusterName)
	if err != nil {
		return err
	}

	cluster, err := client.Steve.SteveType(extensionClusters.ProvisioningSteveResourceType).ByID(provisioningClusterID)
	if err != nil {
		return err
	}

	newCluster := &provv1.Cluster{}
	err = steveV1.ConvertToK8sType(cluster, newCluster)
	if err != nil {
		return err
	}

	firstMachine := nodeList.Data[0]

	logrus.Info("Getting the node IP")
	newNode := &corev1.Node{}
	err = steveV1.ConvertToK8sType(firstMachine.JSONResp, newNode)
	if err != nil {
		return err
	}

	log := ""
	if strings.Contains(newCluster.Spec.KubernetesVersion, "rke2") || strings.Contains(newCluster.Spec.KubernetesVersion, "k3s") {
		_, stevecluster, err := extensionClusters.GetProvisioningClusterByName(client, clusterName, provisioninginput.Namespace)
		if err != nil {
			return err
		}

		sshUser, err := sshkeys.GetSSHUser(client, stevecluster)
		if err != nil {
			return err
		}

		sshNode, err := sshkeys.GetSSHNodeFromMachine(client, sshUser, &firstMachine)
		if err != nil {
			return err
		}

		logrus.Infof("Comand %s", fmt.Sprintf("curl %s:%s", clusterIP, path))

		log, err = sshNode.ExecuteCommand(fmt.Sprintf("curl %s:%s", clusterIP, path))
		if err != nil && !errors.Is(err, &ssh.ExitMissingError{}) {
			return err
		}
	} else {
		logrus.Infof("Comand %s", fmt.Sprintf("curl %s:%s", clusterIP, path))

		execCmd := []string{"curl", fmt.Sprintf("%s:%s", clusterIP, path)}
		log, err = kubectl.Command(client, nil, clusterID, execCmd, "")
		if err != nil {
			return err
		}
	}

	if strings.Contains(log, content) {
		return nil
	}
	return errors.New("Unable to connect to the cluster")
}
