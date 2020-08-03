package certsexpiration

import (
	"context"
	"reflect"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rkecerts"
	"github.com/rancher/rancher/pkg/types/config"
	rkeCluster "github.com/rancher/rke/cluster"
	"github.com/sirupsen/logrus"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

// This controller handles cert expiration for local cluster only
func Register(ctx context.Context, management *config.ManagementContext) {
	c := &certsExpiration{
		clusters:        management.Management.Clusters(""),
		configMapLister: management.Core.ConfigMaps("kube-system").Controller().Lister(),
	}
	management.Management.Clusters("").AddHandler(ctx, "certificate-expiration", c.sync)
}

type certsExpiration struct {
	clusters        v3.ClusterInterface
	configMapLister v1.ConfigMapLister
}

func (c *certsExpiration) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if cluster == nil || cluster.Name != "local" {
		return cluster, nil // We are only checking local cluster
	}
	cm, err := c.configMapLister.Get("kube-system", rkeCluster.FullStateConfigMapName)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return cluster, nil // not an rke cluster, nothing we can do
		}
		return cluster, err
	}
	certBundle, err := rkecerts.CertBundleFromConfig(cm)
	if err != nil {
		return cluster, err
	}
	rkecerts.CleanCertificateBundle(certBundle)

	certsExpInfo := map[string]v32.CertExpiration{}
	for certName, certObj := range certBundle {
		info, err := rkecerts.GetCertExpiration(certObj.CertificatePEM)
		if err != nil {
			logrus.Debugf("failed to get expiration date for certificate [%s] for local cluster: %v", certName, err)
			continue
		}
		certsExpInfo[certName] = info
		err = logCertExpirationWarning(certName, info)
		if err != nil {
			logrus.Warnf("certificate [%s] from local cluster has or will expire and date is corrupted: %v", certName, err)
			continue
		}
	}
	// Update certExpiration on cluster obj in order for it to display in API, and the UI if expiring
	if !reflect.DeepEqual(cluster.Status.CertificatesExpiration, certsExpInfo) {
		cluster.Status.CertificatesExpiration = certsExpInfo
		return c.clusters.Update(cluster)
	}
	return cluster, nil
}

func logCertExpirationWarning(name string, certExp v32.CertExpiration) error {
	date, err := time.Parse(time.RFC3339, certExp.ExpirationDate)
	if err != nil {
		return err
	}
	if time.Now().UTC().After(date) { // warn if expired
		logrus.Warnf("Certificate from local cluster has expired: %s", name)
	} else if time.Now().UTC().AddDate(0, 1, 0).After(date) { // warn if within a month
		logrus.Warnf("Certificate from local cluster will expire soon: %s", name)
	}
	return nil
}
