package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/ingresses"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
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
