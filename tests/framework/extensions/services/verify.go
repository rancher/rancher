package services

import (
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/ingresses"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	noSuchHostSubString = "no such host"
)

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
