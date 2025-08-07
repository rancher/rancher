package cavalidator

import (
	"context"
	"fmt"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/controller"
	mgmtv3controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/condition"
	wcore "github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
)

const (
	CertificateAuthorityValid = condition.Cond("AgentTlsStrictCheck")
	CacertsValid              = "CATTLE_CACERTS_VALID"
	stvAggregationSecretName  = "stv-aggregation"
)

type CertificateAuthorityValidator struct {
	clusterName  string
	clusterCache mgmtv3controllers.ClusterCache
	clusters     mgmtv3controllers.ClusterClient
}

func Register(ctx context.Context, downstream *config.UserContext) {
	// The stv-aggregation secret will never exist in the local cluster, as it is created by cattle-cluster-agent
	if downstream.ClusterName == "local" {
		return
	}

	c := &CertificateAuthorityValidator{
		clusterName:  downstream.ClusterName,
		clusterCache: downstream.Management.Wrangler.Mgmt.Cluster().Cache(),
		clusters:     downstream.Management.Wrangler.Mgmt.Cluster(),
	}

	restConfig := downstream.RESTConfig

	// TODO: Everything except the OnChange should move to where we define
	// UserContext. UserContext should create and start a new controller factory
	// with a `CAValidatorSecret wcorev1.SecretController` field imo.
	opts := &controller.SharedControllerFactoryOptions{
		CacheOptions: &cache.SharedCacheFactoryOptions{
			KindTweakList: map[schema.GroupVersionKind]cache.TweakListOptionsFunc{
				// Only keep a cache for this single Secret
				corev1.SchemeGroupVersion.WithKind("Secret"): func(opts *metav1.ListOptions) {
					opts.FieldSelector = fmt.Sprintf("metadata.namespace=%s,metadata.name=%s", namespace.System, stvAggregationSecretName)
				},
			},
		},
	}
	controllerFactory, err := controller.NewSharedControllerFactoryFromConfigWithOptions(&downstream.RESTConfig, nil, opts)
	if err != nil {
		// TODO: handle error
		return
	}

	genOpts := &generic.FactoryOptions{
		SharedControllerFactory: controllerFactory,
	}
	coreCtrl, err := wcore.NewFactoryFromConfigWithOptions(&restConfig, genOpts)
	if err != nil {
		// TODO: handle error
		return
	}
	controllerFactory.Start(ctx, 1)

	coreCtrl.Core().V1().Secret().OnChange(ctx, "cavalidator-secret", c.onStvAggregationSecret)
	if err != nil {
		// TODO: handle error
		return
	}
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
