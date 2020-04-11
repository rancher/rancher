package clusterapiprovisioner

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"time"

	"github.com/rancher/kontainer-engine/drivers/util"
	"github.com/rancher/norman/controller"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/wrangler"
	v1alpha32 "github.com/rancher/rancher/pkg/wrangler/generated/controllers/cluster.x-k8s.io/v1alpha3"
	apiv32 "github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io/v3"
	v12 "github.com/rancher/types/apis/core/v1"
	apiv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
)

type handler struct {
	CAPIClusterClient        v1alpha32.ClusterController
	RancherClusterClient     apiv32.ClusterClient
	RancherClusterCache      apiv32.ClusterCache
	ClusterEnqueue           func(name string)
	SecretLister             v12.SecretLister
	backoff                  *flowcontrol.Backoff
}

type Clusters struct {
	Clusters        []C `yaml:"clusters"`
	Users           []User `yaml:"users"`
}

type DataCluster struct {
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	Server                   string `yaml:"server,omitempty"`
}

type C struct {
	DataCluster DataCluster `yaml:"cluster"`
}

type User struct {
	Data UserData `yaml:"user"`
}

type UserData struct {
	ClientCertificateData string `yaml:"client-certificate-data"`
	ClientKeyData         string `yaml:"client-key-data"`
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext, manager *clustermanager.Manager) {
	h := &handler{
		CAPIClusterClient:        wContext.V1alpha3.Cluster(),
		RancherClusterClient:     wContext.Mgmt.Cluster(),
		RancherClusterCache:      wContext.Mgmt.Cluster().Cache(),
		ClusterEnqueue:           wContext.Mgmt.Cluster().Enqueue,
		SecretLister:             mgmtCtx.Core.Secrets("").Controller().Lister(),
		backoff:                  flowcontrol.NewBackOff(30*time.Second, 10*time.Minute),
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "clusterapi-provisioner", h.onClusterChange)
}

func (h *handler) onClusterChange(key string, cluster *apiv3.Cluster) (*apiv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	clusterCopy := cluster.DeepCopy()

	if cluster.Status.Driver != "clusterAPI" {
		return cluster, nil
	}

	if cluster.Status.APIEndpoint != "" && cluster.Status.CACert != "" && cluster.Status.ServiceAccountToken != "" {
		return cluster, nil
	}

	apiv3.ClusterConditionProvisioned.CreateUnknownIfNotExists(clusterCopy)

	if apiv3.ClusterConditionWaiting.GetStatus(clusterCopy) == "" {
		apiv3.ClusterConditionWaiting.Unknown(clusterCopy)
	}
	if apiv3.ClusterConditionWaiting.GetMessage(clusterCopy) == "" {
		apiv3.ClusterConditionWaiting.Message(clusterCopy, "Waiting for API to be available")
	}

	if !reflect.DeepEqual(cluster, clusterCopy) {
		var err error

		cluster, err = h.RancherClusterClient.Update(clusterCopy)
		if err != nil {
			return cluster, err
		}
	}

	clusterAPIParentName, _ := cluster.Labels["cattle.io/clusterapi-parent"]
	if clusterAPIParentName == "" {
		return cluster, nil
	}

	clusterAPIParent, err := h.CAPIClusterClient.Get("default", clusterAPIParentName, v1.GetOptions{})
	if err != nil {
		return cluster, err
	}

	switch clusterAPIParent.Status.GetTypedPhase() {
	case v1alpha3.ClusterPhaseProvisioning:
		apiv3.ClusterConditionProvisioned.Unknown(clusterCopy)

		ok, delay := h.backoffFailure(cluster.Name, fmt.Sprintf("%s-wait-provisioned", cluster.Name))
		if ok {
			return cluster, &controller.ForgetError{Err: fmt.Errorf("waiting for clustapi cluster [%s] to provision, delay %v", clusterAPIParentName, delay)}
		}
		return cluster, &controller.ForgetError{Err: fmt.Errorf("waiting for clustapi cluster [%s] to provision", clusterAPIParentName)}
	case v1alpha3.ClusterPhaseProvisioned:
		apiv3.ClusterConditionProvisioned.True(clusterCopy)
	case v1alpha3.ClusterPhaseFailed:
		apiv3.ClusterConditionProvisioned.False(clusterCopy)
		// persist any status changes
		if !reflect.DeepEqual(cluster, clusterCopy) {
			var err error
			cluster, err = h.RancherClusterClient.Update(clusterCopy)
			if err != nil {
				return cluster, err
			}
		}
		return cluster, fmt.Errorf("clusterapi cluster [%s] failed to provision", clusterAPIParentName)
	}

	if !reflect.DeepEqual(cluster, clusterCopy) {
		var err error
		cluster, err = h.RancherClusterClient.Update(clusterCopy)
		if err != nil {
			return cluster, err
		}
	}

	name := fmt.Sprintf("%s-kubeconfig", clusterAPIParentName)
	fmt.Println(name)
	kubeconfig, err := h.SecretLister.Get("default", fmt.Sprintf("%s-kubeconfig", clusterAPIParentName))
	if err != nil {
		if !errors.IsNotFound(err) {
			return cluster, err
		}
		apiv3.ClusterConditionWaiting.IsUnknown(clusterCopy)
		// persist any status changes
		if !reflect.DeepEqual(cluster, clusterCopy) {
			var err error
			cluster, err = h.RancherClusterClient.Update(clusterCopy)
			if err != nil {
				return cluster, err
			}
		}
		ok, delay := h.backoffFailure(cluster.Name, fmt.Sprintf("%s-wait-kubeconfig", cluster.Name))
		if ok {
			return cluster, &controller.ForgetError{Err: fmt.Errorf("waiting for kubeconfig to be available, delay %v", delay)}
		}
		return cluster, &controller.ForgetError{Err: fmt.Errorf("waiting for kubeconfig to be available")}
	}

	into := Clusters{}
	if err := yaml.Unmarshal(kubeconfig.Data["value"], &into); err != nil {
		return cluster, err
	}

	cluster.Status.APIEndpoint = into.Clusters[0].DataCluster.Server
	cluster.Status.CACert = into.Clusters[0].DataCluster.CertificateAuthorityData

	saToken, err := generateServiceAccountToken(into.Clusters[0].DataCluster, into.Users[0].Data)
	if err != nil {
		return cluster, err
	}

	cluster.Status.ServiceAccountToken = saToken
	apiv3.ClusterConditionWaiting.True(cluster)
	return h.RancherClusterClient.Update(cluster)
}

func (h *handler) backoffFailure(clusterName, key string) (bool, time.Duration) {
	if h.backoff.IsInBackOffSinceUpdate(key, time.Now()) {
		go func() {
			time.Sleep(h.backoff.Get(key))
			h.ClusterEnqueue(clusterName)
		}()
		return true, h.backoff.Get(key)
	}
	h.backoff.Next(key, time.Now())
	return false, 0
}
func generateServiceAccountToken(cluster DataCluster, user UserData) (string, error) {
	capem, err := base64.StdEncoding.DecodeString(cluster.CertificateAuthorityData)
	if err != nil {
		return "", fmt.Errorf("error decoding root ca certificate: %v", err)
	}

	key, err := base64.StdEncoding.DecodeString(user.ClientKeyData)
	if err != nil {
		return "", fmt.Errorf("error decoding client key: %v", err)
	}

	cert, err := base64.StdEncoding.DecodeString(user.ClientCertificateData)
	if err != nil {
		return "", fmt.Errorf("error decoding client certificate: %v", err)
	}

	config := &rest.Config{
		Host: cluster.Server,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   capem,
			KeyData:  key,
			CertData: cert,
		},
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("error creating clientset: %v", err)
	}

	_, err = clientset.DiscoveryClient.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get Kubernetes server version: %v", err)
	}

	return util.GenerateServiceAccountToken(clientset)
}
