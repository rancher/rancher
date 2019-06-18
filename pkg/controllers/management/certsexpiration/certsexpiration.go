package certsexpiration

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/kontainer-engine/cluster"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	rkecluster "github.com/rancher/rke/cluster"
	"github.com/rancher/rke/pki"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/cert"
)

type Controller struct {
	ClusterLister  v3.ClusterLister
	ClusterClient  v3.ClusterInterface
	ClusterManager *clustermanager.Manager
	ClusterStore   cluster.PersistentStore
}

func Register(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	c := &Controller{
		ClusterLister:  management.Management.Clusters("").Controller().Lister(),
		ClusterClient:  management.Management.Clusters(""),
		ClusterManager: manager,
		ClusterStore:   clusterprovisioner.NewPersistentStore(management.Core.Namespaces(""), management.Core),
	}
	management.Management.Clusters("").AddHandler(ctx, "certificate-expiration", c.sync)
}

func (c Controller) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if key == "" || cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	if cluster.Spec.RancherKubernetesEngineConfig == nil {
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
	if !reflect.DeepEqual(cluster.Status.CertificatesExpiration, certsExpInfo) {
		cluster.Status.CertificatesExpiration = certsExpInfo
		return c.ClusterClient.Update(cluster.DeepCopy())
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
	clusterContext, err := c.ClusterManager.UserContext(clusterName)
	if err != nil {
		return nil, err
	}
	certs, err := getCertsFromUserCluster(clusterContext)
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

func getCertsFromUserCluster(c *config.UserContext) (map[string]pki.CertificatePKI, error) {
	certs := map[string]pki.CertificatePKI{}
	secrets, err := c.Core.Secrets("").Controller().Lister().List("kube-system", labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, s := range secrets {
		if strings.HasPrefix(s.GetName(), "kube-") &&
			s.Type == corev1.SecretTypeOpaque {
			name := s.GetName()
			secret, err := c.Core.Secrets("").Controller().Lister().Get("kube-system", name)
			if err != nil {
				logrus.Warnf("failed to read secret [%s] from cluster [%s]", name, c.ClusterName)
				continue
			}
			cert, ok := secret.Data["Certificate"]
			if !ok {
				logrus.Debugf("secret [%s] for cluster [%s] doesn't contain a certificate", name, c.ClusterName)
				continue
			}
			certs[name] = pki.CertificatePKI{CertificatePEM: string(cert)}
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
		ExpirationDate: date.String(),
	}, nil
}
