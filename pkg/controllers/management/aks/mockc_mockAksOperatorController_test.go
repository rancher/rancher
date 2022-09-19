//
// CODE GENERATED AUTOMATICALLY WITH github.com/kelveny/mockcompose
// THIS FILE SHOULD NOT BE EDITED BY HAND
//
package aks

import (
	stderrors "errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/rancher/rancher/pkg/controllers/management/clusteroperator"
	"github.com/rancher/rancher/pkg/controllers/management/clusterupstreamrefresher"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/rest"
)

type mockAksOperatorController struct {
	aksOperatorController
	mock.Mock
}

func (e *mockAksOperatorController) setInitialUpstreamSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	logrus.Infof("setting initial upstreamSpec on cluster [%s]", cluster.Name)
	upstreamSpec, err := clusterupstreamrefresher.BuildAKSUpstreamSpec(e.SecretsCache, e.secretClient, cluster)
	if err != nil {
		return cluster, err
	}
	cluster = cluster.DeepCopy()
	cluster.Status.AKSStatus.UpstreamSpec = upstreamSpec
	return e.ClusterClient.Update(cluster)
}

func (e *mockAksOperatorController) generateAndSetServiceAccount(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	restConfig, err := e.getRestConfig(cluster)
	if err != nil {
		return cluster, fmt.Errorf("error getting kube config: %v", err)
	}
	clusterDialer, err := e.ClientDialer.ClusterDialer(cluster.Name)
	if err != nil {
		return cluster, err
	}
	restConfig.Dial = clusterDialer
	saToken, err := clusteroperator.GenerateSAToken(restConfig)
	if err != nil {
		return cluster, fmt.Errorf("error generating service account token: %v", err)
	}
	cluster = cluster.DeepCopy()
	secret, err := secretmigrator.NewMigrator(e.SecretsCache, e.Secrets).CreateOrUpdateServiceAccountTokenSecret(cluster.Status.ServiceAccountTokenSecret, saToken, cluster)
	if err != nil {
		return nil, err
	}
	cluster.Status.ServiceAccountTokenSecret = secret.Name
	cluster.Status.ServiceAccountToken = ""
	return e.ClusterClient.Update(cluster)
}

func (e *mockAksOperatorController) generateSATokenWithPublicAPI(cluster *mgmtv3.Cluster) (string, *bool, error) {
	restConfig, err := e.getRestConfig(cluster)
	if err != nil {
		return "", nil, err
	}
	requiresTunnel := new(bool)
	restConfig.Dial = (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	serviceToken, err := clusteroperator.GenerateSAToken(restConfig)
	if err != nil {
		*requiresTunnel = true
		var dnsError *net.DNSError
		if stderrors.As(err, &dnsError) && !dnsError.IsTemporary {
			return "", requiresTunnel, nil
		}
		var urlError *url.Error
		if stderrors.As(err, &urlError) && urlError.Timeout() {
			return "", requiresTunnel, nil
		}
		requiresTunnel = nil
	}
	return serviceToken, requiresTunnel, err
}

func (m *mockAksOperatorController) getRestConfig(cluster *mgmtv3.Cluster) (*rest.Config, error) {

	_mc_ret := m.Called(cluster)

	var _r0 *rest.Config

	if _rfn, ok := _mc_ret.Get(0).(func(*mgmtv3.Cluster) *rest.Config); ok {
		_r0 = _rfn(cluster)
	} else {
		if _mc_ret.Get(0) != nil {
			_r0 = _mc_ret.Get(0).(*rest.Config)
		}
	}

	var _r1 error

	if _rfn, ok := _mc_ret.Get(1).(func(*mgmtv3.Cluster) error); ok {
		_r1 = _rfn(cluster)
	} else {
		_r1 = _mc_ret.Error(1)
	}

	return _r0, _r1

}
