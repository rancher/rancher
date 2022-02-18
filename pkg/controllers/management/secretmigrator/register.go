package secretmigrator

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

type Migrator struct {
	secretLister v1.SecretLister
	secrets      v1.SecretInterface
}

type handler struct {
	migrator                  *Migrator
	clusters                  v3.ClusterInterface
	notifierLister            v3.NotifierLister
	notifiers                 v3.NotifierInterface
	catalogLister             v3.CatalogLister
	catalogs                  v3.CatalogInterface
	clusterCatalogLister      v3.ClusterCatalogLister
	clusterCatalogs           v3.ClusterCatalogInterface
	projectCatalogLister      v3.ProjectCatalogLister
	projectCatalogs           v3.ProjectCatalogInterface
	projectLister             v3.ProjectLister
	sourceCodeProviderConfigs pv3.SourceCodeProviderConfigInterface
}

func NewMigrator(secretLister v1.SecretLister, secrets v1.SecretInterface) *Migrator {
	return &Migrator{
		secretLister: secretLister,
		secrets:      secrets,
	}
}

func Register(ctx context.Context, management *config.ManagementContext) {
	h := handler{
		migrator: NewMigrator(
			management.Core.Secrets("").Controller().Lister(),
			management.Core.Secrets(""),
		),
		clusters:                  management.Management.Clusters(""),
		notifierLister:            management.Management.Notifiers("").Controller().Lister(),
		notifiers:                 management.Management.Notifiers(""),
		catalogLister:             management.Management.Catalogs("").Controller().Lister(),
		catalogs:                  management.Management.Catalogs(""),
		clusterCatalogLister:      management.Management.ClusterCatalogs("").Controller().Lister(),
		clusterCatalogs:           management.Management.ClusterCatalogs(""),
		projectCatalogLister:      management.Management.ProjectCatalogs("").Controller().Lister(),
		projectCatalogs:           management.Management.ProjectCatalogs(""),
		projectLister:             management.Management.Projects("").Controller().Lister(),
		sourceCodeProviderConfigs: management.Project.SourceCodeProviderConfigs(""),
	}
	management.Management.Clusters("").AddHandler(ctx, "cluster-secret-migrator", h.sync)
	management.Management.Catalogs("").AddHandler(ctx, "catalog-secret-migrator", h.syncCatalog)
}
