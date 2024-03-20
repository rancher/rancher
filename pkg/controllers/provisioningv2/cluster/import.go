package cluster

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/v2/pkg/apply"
	"github.com/rancher/wrangler/v2/pkg/generic"
	"github.com/rancher/wrangler/v2/pkg/yaml"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
)

func (h *handler) createClusterAndDeployAgent(cluster *v1.Cluster, status v1.ClusterStatus) ([]runtime.Object, v1.ClusterStatus, error) {
	objs, status, err := h.createCluster(cluster, status, v3.ClusterSpec{
		ImportedConfig: &v3.ImportedConfig{},
	})
	if err != nil {
		return nil, status, err
	}

	if status.AgentDeployed || status.ClusterName == "" {
		return objs, status, nil
	}

	ok, err := h.deployAgent(cluster, status)
	if err != nil {
		return objs, status, err
	}

	status.AgentDeployed = ok
	return objs, status, nil
}

func (h *handler) deployAgent(cluster *v1.Cluster, status v1.ClusterStatus) (bool, error) {
	if _, err := h.mgmtClusterCache.Get(status.ClusterName); apierror.IsNotFound(err) {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, 2*time.Second)
		// wait until the cluster is created
		return false, nil
	} else if err != nil {
		return false, err
	}

	tokens, err := h.clusterTokenCache.List(status.ClusterName, labels.Everything())
	if err != nil {
		return false, err
	}

	if len(tokens) == 0 {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, 2*time.Second)
		return false, err
	}

	tokenValue := tokens[0].Status.Token
	if tokenValue == "" {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, 2*time.Second)
		return false, nil
	}

	secretName := fmt.Sprintf("%s-kubeconfig", cluster.Spec.ClusterAPIConfig.ClusterName)
	return true, h.deploy(cluster, cluster.Namespace, secretName, tokenValue)
}

func (h *handler) deploy(cluster *v1.Cluster, secretNamespace, secretName string, token string) error {
	secret, err := h.secretCache.Get(secretNamespace, secretName)
	if apierror.IsNotFound(err) {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, 2*time.Second)
		return generic.ErrSkip
	} else if err != nil {
		return err
	}

	if len(secret.Data) == 0 {
		h.clusters.EnqueueAfter(cluster.Namespace, cluster.Name, 2*time.Second)
		return generic.ErrSkip
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return err
	}

	serverURL, cacert := settings.InternalServerURL.Get(), settings.InternalCACerts.Get()

	httpClient, err := h.httpClientForCA(cacert)
	if err != nil {
		return err
	}

	resp, err := httpClient.Get(fmt.Sprintf("%s/v3/import/%s.yaml", serverURL, token))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	objs, err := yaml.ToObjects(resp.Body)
	if err != nil {
		return err
	}

	apply, err := apply.NewForConfig(cfg)
	if err != nil {
		return err
	}

	return apply.
		WithDynamicLookup().
		WithSetID("cluster-agent-setup").
		ApplyObjects(objs...)
}

func (h *handler) httpClientForCA(cacert string) (*http.Client, error) {
	if cacert == "" {
		return http.DefaultClient, nil
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(cacert))

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs: pool,
	}

	return &http.Client{
		Transport: transport,
	}, nil
}
