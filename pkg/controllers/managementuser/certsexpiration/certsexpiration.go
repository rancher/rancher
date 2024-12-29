package certsexpiration

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	rketypes "github.com/rancher/rke/types"

	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/cluster"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/types/config"
	rkecluster "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/services"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	ClusterName   string
	ClusterLister v3.ClusterLister
	ClusterClient v3.ClusterInterface
	ClusterStore  cluster.PersistentStore
	secretLister  interface {
		ListNamespaced(string, metav1.ListOptions) (*corev1.SecretList, error)
	}
}

func Register(ctx context.Context, userContext *config.UserContext) {
	starter := userContext.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, userContext)
		return nil
	})

	clusters := userContext.Management.Management.Clusters("")
	clusters.AddHandler(ctx, "certs-expiration-deferred", func(key string, obj *v32.Cluster) (runtime.Object, error) {
		if obj != nil &&
			obj.Name == userContext.ClusterName &&
			obj.Spec.RancherKubernetesEngineConfig != nil &&
			obj.Status.AppliedSpec.RancherKubernetesEngineConfig != nil {
			return obj, starter()
		}
		return obj, nil
	})
}

func registerDeferred(ctx context.Context, userContext *config.UserContext) {
	c := &Controller{
		ClusterName:   userContext.ClusterName,
		ClusterLister: userContext.Management.Management.Clusters("").Controller().Lister(),
		ClusterClient: userContext.Management.Management.Clusters(""),
		ClusterStore:  clusterprovisioner.NewPersistentStore(userContext.Management.Core.Namespaces(""), userContext.Management.Core, userContext.Management.Management.Clusters("")),
		secretLister:  userContext.Core.Secrets(""),
	}

	userContext.Management.Management.Clusters("").AddHandler(ctx, "certificate-expiration", c.sync)
}

func (c Controller) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if key == "" || cluster == nil || cluster.DeletionTimestamp != nil || cluster.Name != c.ClusterName {
		return cluster, nil
	}

	if cluster.Spec.RancherKubernetesEngineConfig == nil {
		return cluster, nil
	}
	if cluster.Status.AppliedSpec.RancherKubernetesEngineConfig == nil {
		return cluster, nil
	}
	certsExpInfo := map[string]v32.CertExpiration{}

	cluster, err := c.ClusterLister.Get("", key)
	if err != nil {
		return cluster, err
	}
	certBundle, err := c.getClusterCertificateBundle(cluster.Name)
	if err != nil {
		return cluster, err
	}
	for certName, certObj := range certBundle {
		info, err := rkecerts.GetCertExpiration(certObj.CertificatePEM)
		if err != nil {
			logrus.Debugf("failed to get expiration date for certificate [%s] for cluster [%s]:%v", certName, key, err)
			continue
		}
		certsExpInfo[certName] = info
	}
	logrus.Debugf("Checking and deleting unused certificates for cluster %s", cluster.Name)
	deleteUnusedCerts(certsExpInfo, cluster.Status.AppliedSpec.RancherKubernetesEngineConfig)
	if !reflect.DeepEqual(cluster.Status.CertificatesExpiration, certsExpInfo) {
		toUpdate := cluster.DeepCopy()
		toUpdate.Status.CertificatesExpiration = certsExpInfo
		return c.ClusterClient.Update(toUpdate)
	}
	return cluster, nil

}

func (c Controller) getClusterCertificateBundle(clusterName string) (map[string]pki.CertificatePKI, error) {
	// cluster has a state file ?
	currentState, err := getRKECurrentStateFromStore(c.ClusterStore, clusterName)
	if err != nil {
		return nil, err
	}
	if currentState != nil {
		rkecerts.CleanCertificateBundle(currentState.CertificatesBundle)
		return currentState.CertificatesBundle, nil
	}

	// No state file, let's try get the certs from the user cluster
	certs, err := c.getCertsFromUserCluster()
	if err != nil {
		return nil, err
	}
	rkecerts.CleanCertificateBundle(certs)
	return certs, nil
}

func getRKECurrentStateFromStore(store cluster.PersistentStore, clusterName string) (*rkecluster.State, error) {
	cluster, err := store.Get(clusterName)
	if err != nil {
		return nil, err
	}
	var fullState rkecluster.FullState
	stateStr, ok := cluster.Metadata["fullState"]
	if !ok {
		return nil, nil
	}
	err = json.Unmarshal([]byte(stateStr), &fullState)
	if err != nil {
		return nil, err
	}
	return &fullState.CurrentState, nil
}

func (c Controller) getCertsFromUserCluster() (map[string]pki.CertificatePKI, error) {
	certs := map[string]pki.CertificatePKI{}
	secrets, err := c.secretLister.ListNamespaced(metav1.NamespaceSystem, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("type=%s", corev1.SecretTypeOpaque),
	})
	if err != nil {
		return nil, fmt.Errorf("getting certificate secrets for cluster [%s]: %w", c.ClusterName, err)
	}
	for _, secret := range secrets.Items {
		if strings.HasPrefix(secret.GetName(), "kube-") {
			cert, ok := secret.Data["Certificate"]
			if !ok {
				logrus.Debugf("secret [%s] for cluster [%s] doesn't contain a certificate", secret.GetName(), c.ClusterName)
				continue
			}
			certs[secret.GetName()] = pki.CertificatePKI{CertificatePEM: string(cert)}
		}
	}
	return certs, nil
}

// deleteUnusedCerts removes unused certs and cleans up kubelet certs when GenerateServingCertificate is disabled
func deleteUnusedCerts(certsExpInfo map[string]v32.CertExpiration, rancherKubernetesEngineConfig *rketypes.RancherKubernetesEngineConfig) {
	unusedCerts := make(map[string]bool)
	for k := range certsExpInfo {
		if strings.HasPrefix(k, pki.EtcdCertName) || strings.HasPrefix(k, pki.KubeletCertName) {
			unusedCerts[k] = true
		}
	}
	etcdHosts := hosts.NodesToHosts(rancherKubernetesEngineConfig.Nodes, services.ETCDRole)
	allHosts := hosts.NodesToHosts(rancherKubernetesEngineConfig.Nodes, "")
	for _, host := range etcdHosts {
		etcdName := pki.GetCrtNameForHost(host, pki.EtcdCertName)
		delete(unusedCerts, etcdName)
	}
	if pki.IsKubeletGenerateServingCertificateEnabledinConfig(rancherKubernetesEngineConfig) {
		for _, host := range allHosts {
			kubeletName := pki.GetCrtNameForHost(host, pki.KubeletCertName)
			delete(unusedCerts, kubeletName)
		}
	}

	for k := range unusedCerts {
		logrus.Infof("Deleting unused certificate: %s", k)
		delete(certsExpInfo, k)
	}
}
