package certsexpiration

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/kontainer-engine/cluster"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	rkecluster "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/hosts"
	"github.com/rancher/rke/pki"
	"github.com/rancher/rke/pki/cert"
	"github.com/rancher/rke/services"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type Controller struct {
	ClusterName   string
	ClusterLister v3.ClusterLister
	ClusterClient v3.ClusterInterface
	ClusterStore  cluster.PersistentStore
	SecretLister  v1.SecretLister
}

func Register(ctx context.Context, userContext *config.UserContext) {
	c := &Controller{
		ClusterName:   userContext.ClusterName,
		ClusterLister: userContext.Management.Management.Clusters("").Controller().Lister(),
		ClusterClient: userContext.Management.Management.Clusters(""),
		ClusterStore:  clusterprovisioner.NewPersistentStore(userContext.Management.Core.Namespaces(""), userContext.Management.Core),
		SecretLister:  userContext.Core.Secrets("").Controller().Lister(),
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
	certsExpInfo := map[string]v3.CertExpiration{}

	cluster, err := c.ClusterLister.Get("", key)
	if err != nil {
		return cluster, err
	}
	certBundle, err := c.getClusterCertificateBundle(cluster.Name)
	if err != nil {
		return cluster, err
	}
	for certName, certObj := range certBundle {
		info, err := getCertExpiration(certObj.CertificatePEM)
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
		cleanCertificateBundle(currentState.CertificatesBundle)
		return currentState.CertificatesBundle, nil
	}

	// No state file, let's try get the certs from the user cluster
	certs, err := c.getCertsFromUserCluster()
	if err != nil {
		return nil, err
	}
	cleanCertificateBundle(certs)
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
	secrets, err := c.SecretLister.List("kube-system", labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, secret := range secrets {
		if strings.HasPrefix(secret.GetName(), "kube-") &&
			secret.Type == corev1.SecretTypeOpaque {
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

func cleanCertificateBundle(certs map[string]pki.CertificatePKI) {
	for name := range certs {
		if strings.Contains(name, "client") ||
			strings.Contains(name, "token") ||
			strings.Contains(name, "header") ||
			strings.Contains(name, "admin") {
			delete(certs, name)
		}
	}
}

func getCertificateExpDate(c string) (*time.Time, error) {
	certs, err := cert.ParseCertsPEM([]byte(c))
	if err != nil {
		return nil, err
	}
	return &certs[0].NotAfter, nil
}

func getCertExpiration(c string) (v3.CertExpiration, error) {
	date, err := getCertificateExpDate(c)
	if err != nil {
		return v3.CertExpiration{}, err
	}
	return v3.CertExpiration{
		ExpirationDate: date.Format(time.RFC3339),
	}, nil
}

//deleteUnusedCerts removes unused certs and cleans up kubelet certs when GenerateServingCertificate is disabled
func deleteUnusedCerts(certsExpInfo map[string]v3.CertExpiration, rancherKubernetesEngineConfig *v3.RancherKubernetesEngineConfig) {
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
