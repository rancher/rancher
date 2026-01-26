package auth

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/auth/globalroles"
	"github.com/rancher/rancher/pkg/controllers/management/auth/project_cluster"
	"github.com/rancher/rancher/pkg/controllers/management/auth/roletemplates"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func RegisterWranglerIndexers(config *wrangler.Context) {
	config.RBAC.ClusterRoleBinding().Cache().AddIndexer(rbByRoleAndSubjectIndex, rbByClusterRoleAndSubject)
	config.RBAC.ClusterRoleBinding().Cache().AddIndexer(membershipBindingOwnerIndex, func(obj *rbacv1.ClusterRoleBinding) ([]string, error) {
		return indexByMembershipBindingOwner(obj)
	})

	config.RBAC.RoleBinding().Cache().AddIndexer(rbByOwnerIndex, rbByOwner)
	config.RBAC.RoleBinding().Cache().AddIndexer(rbByRoleAndSubjectIndex, rbByRoleAndSubject)
	config.RBAC.RoleBinding().Cache().AddIndexer(membershipBindingOwnerIndex, func(obj *rbacv1.RoleBinding) ([]string, error) {
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

	roletemplates.RegisterIndexers(scaledContext.Wrangler)

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
	ac := newAuthConfigController(management, clusterManager.ScaledContext)
	ua := newUserAttributeController(management.WithAgent(userAttributeController))
	s := newAuthSettingController(ctx, management)
	rt := newRoleTemplateLifecycle(management, clusterManager)
	prtbServiceAccountFinder := newPRTBServiceAccountController(management)

	management.Management.Clusters("").AddHandler(ctx, project_cluster.ClusterCreateController, c.Sync)
	management.Management.Projects("").AddHandler(ctx, project_cluster.ProjectCreateController, p.Sync)
	management.Management.ProjectRoleTemplateBindings("").AddHandler(ctx, prtbServiceAccountControllerName, prtbServiceAccountFinder.sync)
	management.Management.Tokens("").AddHandler(ctx, tokenController, n.sync)
	management.Management.AuthConfigs("").AddHandler(ctx, authConfigControllerName, ac.sync)
	management.Management.UserAttributes("").AddHandler(ctx, userAttributeController, ua.sync)
	management.Management.Settings("").AddHandler(ctx, authSettingController, s.sync)
	globalroles.Register(ctx, management, clusterManager)

	// Register aggregated-roletemplate controllers
	roletemplates.Register(ctx, management, clusterManager)

	// Register non aggregated-roletemplate controllers
	management.Management.ClusterRoleTemplateBindings("").AddLifecycle(ctx, ctrbMGMTController, crtb)
	management.Management.ProjectRoleTemplateBindings("").AddLifecycle(ctx, ptrbMGMTController, prtb)
	management.Management.RoleTemplates("").AddLifecycle(ctx, roleTemplateLifecycleName, rt)

	aggregationEnqueuer := aggregationEnqueuer{
		crtbCache: management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		prtbCache: management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		rtCache:   management.Wrangler.Mgmt.RoleTemplate().Cache(),
	}
	relatedresource.WatchClusterScoped(ctx, "aggregation-feature-rt-enqueuer", aggregationEnqueuer.enqueueRoleTemplates, management.Wrangler.Mgmt.RoleTemplate(), management.Wrangler.Mgmt.Feature())
	relatedresource.Watch(ctx, "aggregation-feature-crtb-enqueuer", aggregationEnqueuer.enqueueCRTBs, management.Wrangler.Mgmt.ClusterRoleTemplateBinding(), management.Wrangler.Mgmt.Feature())
	relatedresource.Watch(ctx, "aggregation-feature-prtb-enqueuer", aggregationEnqueuer.enqueuePRTBs, management.Wrangler.Mgmt.ProjectRoleTemplateBinding(), management.Wrangler.Mgmt.Feature())

	management.Management.Users("").AddLifecycle(ctx, userController, u)
}

func RegisterLate(ctx context.Context, management *config.ManagementContext) {
	p := project_cluster.NewProjectLifecycle(management)
	c := project_cluster.NewClusterLifecycle(management)
	management.Management.Projects("").AddLifecycle(ctx, project_cluster.ProjectRemoveController, p)
	management.Management.Clusters("").AddLifecycle(ctx, project_cluster.ClusterRemoveController, c)
}

type aggregationEnqueuer struct {
	crtbCache mgmtv3.ClusterRoleTemplateBindingCache
	prtbCache mgmtv3.ProjectRoleTemplateBindingCache
	rtCache   mgmtv3.RoleTemplateCache
}

func isFeatureAggregation(obj runtime.Object) bool {
	if obj == nil {
		return false
	}
	feature, ok := obj.(*v3.Feature)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a feature", obj)
		return false
	}
	return feature.Name == "aggregated-roletemplates"
}

// enqueueAggregationResources is a helper function to enqueue resources when the "aggregated-roletemplates" feature is toggled
func enqueueAggregationResources[T generic.RuntimeMetaObject](obj runtime.Object, listFunc func() ([]T, error)) ([]relatedresource.Key, error) {
	if !isFeatureAggregation(obj) {
		return nil, nil
	}
	objs, err := listFunc()
	if err != nil {
		return nil, err
	}
	keys := make([]relatedresource.Key, 0, len(objs))
	for _, o := range objs {
		metaObj, err := meta.Accessor(o)
		if err != nil {
			return nil, err
		}
		keys = append(keys, relatedresource.Key{Name: metaObj.GetName(), Namespace: metaObj.GetNamespace()})
	}
	return keys, nil
}

func (a *aggregationEnqueuer) enqueueCRTBs(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	return enqueueAggregationResources(obj, func() ([]*v3.ClusterRoleTemplateBinding, error) { return a.crtbCache.List("", labels.NewSelector()) })
}

func (a *aggregationEnqueuer) enqueuePRTBs(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	return enqueueAggregationResources(obj, func() ([]*v3.ProjectRoleTemplateBinding, error) { return a.prtbCache.List("", labels.NewSelector()) })
}

func (a *aggregationEnqueuer) enqueueRoleTemplates(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	return enqueueAggregationResources(obj, func() ([]*v3.RoleTemplate, error) { return a.rtCache.List(labels.NewSelector()) })
}
