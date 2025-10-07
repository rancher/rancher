package cavalidator

import (
	"context"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/rancher/pkg/controllers"
	mgmtv3controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/condition"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
)

const (
	CertificateAuthorityValid = condition.Cond("AgentTlsStrictCheck")
	CacertsValid              = "CATTLE_CACERTS_VALID"

	stvAggregationSecretNamespace = namespace.System
	stvAggregationSecretName      = "stv-aggregation"
)

type CertificateAuthorityValidator struct {
	clusterName  string
	clusterCache mgmtv3controllers.ClusterCache
	clusters     mgmtv3controllers.ClusterClient
}

func Register(ctx context.Context, downstream *config.UserContext) error {
	// The stv-aggregation secret will never exist in the local cluster, as it is created by cattle-cluster-agent
	if downstream.ClusterName == "local" {
		return nil
	}

	// We want to avoid keeping in the cache all Secrets so we use a separate controller factory that only watches a single Secret
	//
	// The default controller factory restricts downstream Secret caches to the impersonation namespace only, see https://github.com/rancher/rancher/issues/46827
	clientFactory := downstream.ControllerFactory.SharedCacheFactory().SharedClientFactory()
	secrets, controllerFactory := newDedicatedSecretsController(clientFactory)
	if err := downstream.RegisterExtraControllerFactory("cavalidator", controllerFactory); err != nil {
		return err
	}

	c := &CertificateAuthorityValidator{
		clusterName:  downstream.ClusterName,
		clusterCache: downstream.Management.Wrangler.Mgmt.Cluster().Cache(),
		clusters:     downstream.Management.Wrangler.Mgmt.Cluster(),
	}

	secrets.OnChange(ctx, "cavalidator-secret", c.onStvAggregationSecret)
	return nil
}

func (c *CertificateAuthorityValidator) onStvAggregationSecret(key string, obj *corev1.Secret) (*corev1.Secret, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	return obj, retry.RetryOnConflict(retry.DefaultRetry, func() error {
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

		_, err = c.clusters.Update(mgmtCluster)
		return err
	})
}

func newDedicatedSecretsController(clientFactory client.SharedClientFactory) (corecontrollers.SecretController, controller.SharedControllerFactory) {
	// Create a new cache factory that restricts secrets to a single namespace and name
	fieldSelector := fields.AndSelectors(
		fields.OneTermEqualSelector("metadata.namespace", stvAggregationSecretNamespace),
		fields.OneTermEqualSelector("metadata.name", stvAggregationSecretName),
	).String()
	cacheFactory := cache.NewSharedCachedFactory(clientFactory, &cache.SharedCacheFactoryOptions{
		KindTweakList: map[schema.GroupVersionKind]cache.TweakListOptionsFunc{
			corev1.SchemeGroupVersion.WithKind("Secret"): func(opts *metav1.ListOptions) {
				opts.FieldSelector = fieldSelector
			},
		},
	})

	controllerFactory := controller.NewSharedControllerFactory(cacheFactory, controllers.GetOptsFromEnv(controllers.User))
	return corecontrollers.New(controllerFactory).Secret(), controllerFactory
}
