package auth

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/auth/globalroles"
	"github.com/rancher/rancher/pkg/controllers/management/auth/project_cluster"
	"github.com/rancher/rancher/pkg/controllers/management/auth/roletemplates"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

func RegisterWranglerIndexers(config *wrangler.Context) {
	config.RBAC.ClusterRoleBinding().Cache().AddIndexer(rbByRoleAndSubjectIndex, rbByClusterRoleAndSubject)
	config.RBAC.ClusterRoleBinding().Cache().AddIndexer(membershipBindingOwnerIndex, func(obj *v1.ClusterRoleBinding) ([]string, error) {
		return indexByMembershipBindingOwner(obj)
	})

	config.RBAC.RoleBinding().Cache().AddIndexer(rbByOwnerIndex, rbByOwner)
	config.RBAC.RoleBinding().Cache().AddIndexer(rbByRoleAndSubjectIndex, rbByRoleAndSubject)
	config.RBAC.RoleBinding().Cache().AddIndexer(membershipBindingOwnerIndex, func(obj *v1.RoleBinding) ([]string, error) {
		return indexByMembershipBindingOwner(obj)
	})
}

func RegisterIndexers(scaledContext *config.ScaledContext) error {
	prtbInformer := scaledContext.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbByRoleTemplateIndex: prtbByRoleTemplate,
		prtbByUserRefKey:        prtbByUserRefFunc,
	}
	if err := prtbInformer.AddIndexers(prtbIndexers); err != nil {
		return err
	}

	crtbInformer := scaledContext.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		crtbByRoleTemplateIndex: crtbByRoleTemplate,
		crtbByUserRefKey:        crtbByUserRefFunc,
	}
	if err := crtbInformer.AddIndexers(crtbIndexers); err != nil {
		return err
	}

	tokenInformer := scaledContext.Management.Tokens("").Controller().Informer()
	if err := tokenInformer.AddIndexers(map[string]cache.IndexFunc{
		tokenByUserRefKey: tokenByUserRefFunc,
	}); err != nil {
		return err
	}

	grbInformer := scaledContext.Management.GlobalRoleBindings("").Controller().Informer()
	return grbInformer.AddIndexers(map[string]cache.IndexFunc{
		grbByUserRefKey: grbByUserRefFunc,
	})
}

func RegisterEarly(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	prtb, crtb := newRTBLifecycles(management.WithAgent("mgmt-auth-crtb-prtb-controller"))
	p := project_cluster.NewProjectLifecycle(management)
	c := project_cluster.NewClusterLifecycle(management)
	u := newUserLifecycle(management, clusterManager)
	n := newTokenController(management.WithAgent(tokenController))
	ac := newAuthConfigController(ctx, management, clusterManager.ScaledContext)
	ua := newUserAttributeController(management.WithAgent(userAttributeController))
	s := newAuthSettingController(ctx, management)
	rt := newRoleTemplateLifecycle(management, clusterManager)
	grbLegacy := newLegacyGRBCleaner(management)
	rtLegacy := newLegacyRTCleaner(management)
	prtbServiceAccountFinder := newPRTBServiceAccountController(management)

	management.Management.Clusters("").AddHandler(ctx, project_cluster.ClusterCreateController, c.Sync)
	management.Management.Projects("").AddHandler(ctx, project_cluster.ProjectCreateController, p.Sync)
	management.Management.ProjectRoleTemplateBindings("").AddHandler(ctx, prtbServiceAccountControllerName, prtbServiceAccountFinder.sync)
	management.Management.Tokens("").AddHandler(ctx, tokenController, n.sync)
	management.Management.AuthConfigs("").AddHandler(ctx, authConfigControllerName, ac.sync)
	management.Management.UserAttributes("").AddHandler(ctx, userAttributeController, ua.sync)
	management.Management.Settings("").AddHandler(ctx, authSettingController, s.sync)
	management.Management.GlobalRoleBindings("").AddHandler(ctx, "legacy-grb-cleaner", grbLegacy.sync)
	management.Management.RoleTemplates("").AddHandler(ctx, "legacy-rt-cleaner", rtLegacy.sync)
	globalroles.Register(ctx, management, clusterManager)

	if features.AggregatedRoleTemplates.Enabled() {
		roletemplates.Register(ctx, management)
	} else {
		management.Management.ClusterRoleTemplateBindings("").AddLifecycle(ctx, ctrbMGMTController, crtb)
		management.Management.ProjectRoleTemplateBindings("").AddLifecycle(ctx, ptrbMGMTController, prtb)
		management.Management.RoleTemplates("").AddLifecycle(ctx, roleTemplateLifecycleName, rt)
	}
	management.Management.Users("").AddLifecycle(ctx, userController, u)
}

func RegisterLate(ctx context.Context, management *config.ManagementContext) {
	p := project_cluster.NewProjectLifecycle(management)
	c := project_cluster.NewClusterLifecycle(management)
	management.Management.Projects("").AddLifecycle(ctx, project_cluster.ProjectRemoveController, p)
	management.Management.Clusters("").AddLifecycle(ctx, project_cluster.ClusterRemoveController, c)
}
