package cavalidator

import (
	"context"

	mgmtv3controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/watcher"

	"github.com/rancher/wrangler/v3/pkg/condition"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
)

const (
	CertificateAuthorityValid = condition.Cond("AgentTlsStrictCheck")
	CacertsValid              = "CATTLE_CACERTS_VALID"
)

type CertificateAuthorityValidator struct {
	clusterName  string
	clusterCache mgmtv3controllers.ClusterCache
	clusters     mgmtv3controllers.ClusterClient
}

func Register(ctx context.Context, downstream *config.UserContext) {
	if downstream.ClusterName == "local" {
		return
	}

	c := &CertificateAuthorityValidator{
		clusterName:  downstream.ClusterName,
		clusterCache: downstream.Management.Wrangler.Mgmt.Cluster().Cache(),
		clusters:     downstream.Management.Wrangler.Mgmt.Cluster(),
	}

	// Downstream Secret caches are restricted to the impersonation namespace only, see https://github.com/rancher/rancher/issues/46827
	// Use a single Watch instead of relying on full-blown informers
	watcher.OnChange(downstream.Corew.Secret(), ctx, "cavalidator-secret", c.onStvAggregationSecret,
		watcher.WithNamespace(namespace.System),
		watcher.WithFieldSelector(map[string]string{"metadata.name": "stv-aggregation"}),
	)
}

func (c *CertificateAuthorityValidator) onStvAggregationSecret(_ string, obj *corev1.Secret) (*corev1.Secret, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	return nil, retry.RetryOnConflict(retry.DefaultRetry, func() error {
		mgmtCluster, err := c.clusterCache.Get(c.clusterName)
		if err != nil {
			return err
		}
		mgmtCluster = mgmtCluster.DeepCopy()

		if string(obj.Data[CacertsValid]) == "true" && len(obj.Data["ca.crt"]) != 0 {
			if CertificateAuthorityValid.IsTrue(mgmtCluster) {
				return nil
			}
			CertificateAuthorityValid.True(mgmtCluster)
		} else if string(obj.Data[CacertsValid]) == "false" {
			if CertificateAuthorityValid.IsFalse(mgmtCluster) {
				return nil
			}
			CertificateAuthorityValid.False(mgmtCluster)
		} else {
			if CertificateAuthorityValid.IsUnknown(mgmtCluster) {
				return nil
			}
			CertificateAuthorityValid.Unknown(mgmtCluster)
		}

		_, err = c.clusters.UpdateStatus(mgmtCluster)
		return err
	})
}
