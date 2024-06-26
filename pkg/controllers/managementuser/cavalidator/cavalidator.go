package cavalidator

import (
	"context"

	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/condition"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	CertificateAuthorityValid = condition.Cond("AgentTlsStrictCheck")
	CacertsValid              = "CATTLE_CACERTS_VALID"
)

type CertificateAuthorityValidator struct {
	clusterName   string
	clusterLister v3.ClusterLister
	clusters      v3.ClusterInterface
	secrets       corev1.SecretController
}

func Register(ctx context.Context, downstream *config.UserContext) {
	c := &CertificateAuthorityValidator{
		clusterName:   downstream.ClusterName,
		clusterLister: downstream.Management.Management.Clusters("").Controller().Lister(),
		clusters:      downstream.Management.Management.Clusters(""),
		secrets:       downstream.Core.Secrets(namespace.System).Controller(),
	}

	c.secrets.AddHandler(ctx, "cavalidator-secret", c.onStvAggregationSecret)
}

func (c *CertificateAuthorityValidator) onStvAggregationSecret(_ string, obj *corev1.Secret) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}

	if obj.Name == "stv-aggregation" && obj.Namespace == namespace.System {
		mgmtCluster, err := c.clusterLister.Get("", c.clusterName)
		if err != nil {
			return obj, err
		}
		if string(obj.Data[CacertsValid]) == "true" && len(obj.Data["ca.crt"]) != 0 {
			if !CertificateAuthorityValid.IsTrue(mgmtCluster) {
				newMgmtCluster := mgmtCluster.DeepCopy()
				CertificateAuthorityValid.True(newMgmtCluster)
				_, err = c.clusters.Update(newMgmtCluster)
				if err != nil {
					return obj, err
				}
			}
			return obj, nil
		}
		newMgmtCluster := mgmtCluster.DeepCopy()
		if string(obj.Data[CacertsValid]) == "false" {
			CertificateAuthorityValid.False(newMgmtCluster)
		} else {
			CertificateAuthorityValid.Unknown(newMgmtCluster)
		}
		_, err = c.clusters.Update(newMgmtCluster)
		if err != nil {
			return obj, err
		}
	}

	return obj, nil
}
