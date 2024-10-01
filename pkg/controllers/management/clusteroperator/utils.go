package clusteroperator

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/catalog/manager"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/systemaccount"
	typesDialer "github.com/rancher/rancher/pkg/types/config/dialer"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/client-go/util/retry"
)

const (
	localCluster = "local"
	systemNS     = "cattle-system"
)

type OperatorController struct {
	ClusterEnqueueAfter  func(name string, duration time.Duration)
	SecretsCache         wranglerv1.SecretCache
	Secrets              corev1.SecretInterface
	TemplateCache        v3.CatalogTemplateCache
	ProjectCache         v3.ProjectCache
	AppLister            projectv3.AppLister
	AppClient            projectv3.AppInterface
	NsClient             corev1.NamespaceInterface
	ClusterClient        v3.ClusterClient
	CatalogManager       manager.CatalogManager
	SystemAccountManager *systemaccount.Manager
	DynamicClient        dynamic.NamespaceableResourceInterface
	ClientDialer         typesDialer.Factory
	Discovery            discovery.DiscoveryInterface
}

func (e *OperatorController) SetUnknown(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsUnknown(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.Unknown(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.ClusterClient.Update(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, nil
}

func (e *OperatorController) SetTrue(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsTrue(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.True(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.ClusterClient.Update(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, nil
}

func (e *OperatorController) SetFalse(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsFalse(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.False(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.ClusterClient.Update(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, nil
}

// RecordCAAndAPIEndpoint reads the cluster config's secret once available. The CA cert and API endpoint are then copied to the cluster status.
func (e *OperatorController) RecordCAAndAPIEndpoint(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2,
		Jitter:   0,
		Steps:    6,
		Cap:      20 * time.Second,
	}

	var caSecret *corev1.Secret
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		caSecret, err = e.SecretsCache.Get(namespace.GlobalNamespace, cluster.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return false, err
			}
			logrus.Infof("waiting for cluster [%s] data needed to generate service account token", cluster.Name)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return cluster, fmt.Errorf("failed waiting for cluster [%s] secret: %s", cluster.Name, err)
	}

	apiEndpoint := string(caSecret.Data["endpoint"])
	if !strings.HasPrefix(apiEndpoint, "https://") {
		apiEndpoint = "https://" + apiEndpoint
	}
	caCert, err := addAdditionalCA(e.SecretsCache, string(caSecret.Data["ca"]))
	if err != nil {
		return cluster, err
	}
	if cluster.Status.APIEndpoint == apiEndpoint && cluster.Status.CACert == caCert {
		return cluster, nil
	}

	var currentCluster *mgmtv3.Cluster
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentCluster, err = e.ClusterClient.Get(cluster.Name, v1.GetOptions{})
		if err != nil {
			return err
		}
		currentCluster.Status.APIEndpoint = apiEndpoint
		currentCluster.Status.CACert = caCert
		currentCluster, err = e.ClusterClient.Update(currentCluster)
		return err
	})

	return currentCluster, err
}

// checkCRDReady checks whether necessary CRD(AKSConfig/EKSConfig/GKEConfig), has been created yet
func (e *OperatorController) CheckCrdReady(cluster *mgmtv3.Cluster, clusterType string) (*mgmtv3.Cluster, error) {
	resources, err := e.Discovery.ServerResourcesForGroupVersion(fmt.Sprintf("%s.cattle.io/v1", clusterType))
	if err != nil && !errors.IsNotFound(err) {
		return cluster, err
	}
	if errors.IsNotFound(err) || len(resources.APIResources) == 0 {
		cluster, err = e.SetUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, fmt.Sprintf("Waiting on %s crd to be initialized", clusterType))
		if err != nil {
			return cluster, err
		}
		return cluster, fmt.Errorf("waiting %s crd to be initialized, cluster: %v", clusterType, cluster.Name)
	}
	return cluster, nil
}

type TransportConfigOption func(*transport.Config)

func WithDialHolder(holder *transport.DialHolder) TransportConfigOption {
	return func(cfg *transport.Config) {
		cfg.DialHolder = holder
	}
}

func NewClientSetForConfig(config *rest.Config, opts ...TransportConfigOption) (*kubernetes.Clientset, error) {
	transportConfig, err := config.TransportConfig()
	if err != nil {
		return nil, err
	}
	for _, opt := range opts {
		opt(transportConfig)
	}

	rt, err := transport.New(transportConfig)
	if err != nil {
		return nil, err
	}

	var httpClient *http.Client
	if rt != http.DefaultTransport || config.Timeout > 0 {
		httpClient = &http.Client{
			Transport: rt,
			Timeout:   config.Timeout,
		}
	} else {
		httpClient = http.DefaultClient
	}

	return kubernetes.NewForConfigAndClient(config, httpClient)
}

func addAdditionalCA(secretsCache wranglerv1.SecretCache, caCert string) (string, error) {
	additionalCA, err := getAdditionalCA(secretsCache)
	if err != nil {
		return caCert, err
	}
	if additionalCA == nil {
		return caCert, nil
	}

	caBytes, err := base64.StdEncoding.DecodeString(caCert)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(append(caBytes, additionalCA...)), nil
}

func getAdditionalCA(secretsCache wranglerv1.SecretCache) ([]byte, error) {
	secret, err := secretsCache.Get(namespace.System, "tls-ca-additional")
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if secret == nil {
		return nil, nil
	}

	return secret.Data["ca-additional.pem"], nil
}
