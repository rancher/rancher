package secretmigrator

import (
	"context"

	provv1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Migrator struct {
	secretLister v1.SecretLister
	secrets      v1.SecretInterface
}

type authConfigsClient interface {
	Get(name string, opts metav1.GetOptions) (runtime.Object, error)
	Update(name string, o runtime.Object) (runtime.Object, error)
}

type handler struct {
	migrator                 *Migrator
	authConfigLister         v3.AuthConfigLister
	clusters                 v3.ClusterInterface
	provisioningClusters     provv1.ClusterController
	clusterTemplateRevisions v3.ClusterTemplateRevisionInterface
	projectLister            v3.ProjectLister
	// AuthConfigs contain internal-only fields that deal with various auth providers.
	// Those fields are not present everywhere, nor are they defined in the CRD. Given
	// that, the regular client will "eat" those internal-only fields, so in this case, we use
	// the unstructured client, losing some validation, but gaining the flexibility we require.
	authConfigs authConfigsClient
}

func NewMigrator(secretLister v1.SecretLister, secrets v1.SecretInterface) *Migrator {
	return &Migrator{
		secretLister: secretLister,
		secrets:      secrets,
	}
}

func Register(ctx context.Context, management *config.ManagementContext) {
	management = management.WithAgent("secret-migrator")
	h := handler{
		migrator: NewMigrator(
			management.Core.Secrets("").Controller().Lister(),
			management.Core.Secrets(""),
		),
		authConfigs:              management.Management.AuthConfigs("").ObjectClient().UnstructuredClient(),
		authConfigLister:         management.Management.AuthConfigs("").Controller().Lister(),
		clusters:                 management.Management.Clusters(""),
		provisioningClusters:     management.Wrangler.Provisioning.Cluster(),
		clusterTemplateRevisions: management.Management.ClusterTemplateRevisions(""),
		projectLister:            management.Management.Projects("").Controller().Lister(),
	}
	management.Management.AuthConfigs("").AddHandler(ctx, "authconfigs-secret-migrator", h.syncAuthConfig)
	management.Management.Clusters("").AddHandler(ctx, "cluster-secret-migrator", h.sync)
	management.Management.ClusterTemplateRevisions("").AddHandler(ctx, "clustertemplaterevision-secret-migrator", h.syncTemplate)

	management.Wrangler.Provisioning.Cluster().OnChange(ctx, "harvester-secret-migrator", h.syncHarvesterCloudConfig)
	management.Wrangler.Provisioning.Cluster().OnRemove(ctx, "cloud-config-secret-remover", h.cloudConfigSecretRemover)
}
