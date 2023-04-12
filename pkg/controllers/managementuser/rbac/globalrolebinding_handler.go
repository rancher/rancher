package rbac

import (
	"fmt"

	"github.com/rancher/norman/types/slice"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	grbByUserAndRoleIndex = "authz.cluster.cattle.io/grb-by-user-and-role"
	grbHandlerName        = "grb-cluster-sync"
	sourceKey             = "field.cattle.io/source"
)

func RegisterIndexers(scaledContext *config.ScaledContext) error {
	informer := scaledContext.Management.GlobalRoleBindings("").Controller().Informer()
	indexers := map[string]cache.IndexFunc{
		grbByUserAndRoleIndex: grbByUserAndRole,
		grbByRoleIndex:        grbByRole,
	}
	if err := informer.AddIndexers(indexers); err != nil {
		return err
	}

	// Add cache informer to project role template bindings
	prtbInformer := scaledContext.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	prtbIndexers := map[string]cache.IndexFunc{
		prtbByProjectIndex:               prtbByProjectName,
		prtbByProjecSubjectIndex:         prtbByProjectAndSubject,
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
		prtbByUIDIndex:                   prtbByUID,
		prtbByNsAndNameIndex:             prtbByNsName,
		rtbByClusterAndUserIndex:         rtbByClusterAndUserNotDeleting,
	}
	if err := prtbInformer.AddIndexers(prtbIndexers); err != nil {
		return err
	}

	crtbInformer := scaledContext.Management.ClusterRoleTemplateBindings("").Controller().Informer()
	crtbIndexers := map[string]cache.IndexFunc{
		rtbByClusterAndRoleTemplateIndex: rtbByClusterAndRoleTemplateName,
		rtbByClusterAndUserIndex:         rtbByClusterAndUserNotDeleting,
	}
	return crtbInformer.AddIndexers(crtbIndexers)
}

func newGlobalRoleBindingHandler(workload *config.UserContext) v3.GlobalRoleBindingHandlerFunc {

	h := &grbHandler{
		clusterName:         workload.ClusterName,
		clusterRoleBindings: workload.RBAC.ClusterRoleBindings(""),
		crbLister:           workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		// The following clients/controllers all point at the management cluster
		crtbCache: workload.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		crtbCtrl:  workload.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		grbCache:  workload.Management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
		grLister:  workload.Management.Management.GlobalRoles("").Controller().Lister(),
	}

	return h.sync
}

// grbHandler ensures the global admins have full access to every cluster. If a globalRoleBinding is created that uses
// the admin role, then the user in that binding gets a clusterRoleBinding in every user cluster to the cluster-admin role
type grbHandler struct {
	clusterName         string
	clusterRoleBindings rbacv1.ClusterRoleBindingInterface
	crbLister           rbacv1.ClusterRoleBindingLister
	grbCache            mgmtv3.GlobalRoleBindingCache
	crtbCache           mgmtv3.ClusterRoleTemplateBindingCache
	crtbCtrl            mgmtv3.ClusterRoleTemplateBindingClient
	grLister            v3.GlobalRoleLister
}

func (c *grbHandler) sync(key string, obj *apisv3.GlobalRoleBinding) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	if obj.GlobalRoleName == rbac.GlobalRestrictedAdmin {
		// restricted admin only needs to get a CRTB to cluster-owner RoleTemplate
		return obj, c.ensureClusterOwnerTemplateBinding(obj)
	}

	isAdmin, err := c.isAdminRole(obj.GlobalRoleName)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		return obj, nil
	}

	logrus.Debugf("%v is an admin role", obj.GlobalRoleName)

	return obj, c.ensureClusterAdminBinding(obj)
}

// ensureClusterAdminBinding creates a ClusterRoleBinding for GRB subject to
// the Kubernetes "cluster-admin" ClusterRole in the downstream cluster.
func (c *grbHandler) ensureClusterAdminBinding(obj *apisv3.GlobalRoleBinding) error {
	bindingName := rbac.GrbCRBName(obj)
	_, err := c.crbLister.Get("", bindingName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get ClusterRoleBinding '%s' from the cache: %w", bindingName, err)
	}

	if err == nil {
		// binding exists, nothing to do
		return nil
	}

	_, err = c.clusterRoleBindings.Create(&v12.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: bindingName,
		},
		Subjects: []v12.Subject{rbac.GetGRBSubject(obj)},
		RoleRef: v12.RoleRef{
			Name: "cluster-admin",
			Kind: "ClusterRole",
		},
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create ClusterRoleBinding '%s' for admin in downstream '%s': %w", bindingName, c.clusterName, err)
	}
	return nil
}

// ensureClusterOwnerTemplateBinding creates a ClusterRoleTemplateBinding for the GRB subject to
// the "cluster-owner" ClusterRoleTemplate the downstream cluster. If the cluster is the local cluster no action is performed.
func (c *grbHandler) ensureClusterOwnerTemplateBinding(obj *apisv3.GlobalRoleBinding) error {
	if c.clusterName == "local" {
		// Do not sync restricted-admin to the local cluster as 'cluster-admin'
		return nil
	}
	crtbName := name.SafeConcatName(rbac.GetGRBTargetKey(obj), "restricted-admin", "cluster-owner")
	_, err := c.crtbCache.Get(c.clusterName, crtbName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get CRTB '%s' from cache: %w", crtbName, err)
	}
	if err == nil {
		// CRTB was already created.
		// we do not need to check for equivalence between the current CRTB and the desired CRTB
		// this is because the fields we care about can not be modified
		return nil
	}

	// add the restricted admin user as a member of the downstream cluster
	// by creating a CRTB in the local custer in the namespace named after the downstream cluster.
	crtb := apisv3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crtbName,
			Namespace: c.clusterName,
			Labels:    map[string]string{sourceKey: grbHandlerName},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: obj.APIVersion,
					Kind:       obj.Kind,
					Name:       obj.Name,
					UID:        obj.UID,
				},
			},
		},
		ClusterName:      c.clusterName,
		RoleTemplateName: "cluster-owner",
	}

	// CRTBs must contain either user or group information but not both.
	// we will attempt to first use the userName then if not assign the groupName.
	if obj.UserName != "" {
		crtb.UserName = obj.UserName
	} else {
		crtb.GroupPrincipalName = obj.GroupPrincipalName
	}

	_, err = c.crtbCtrl.Create(&crtb)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create a CRTB '%s': %w", crtbName, err)
	}

	return nil
}

// isAdminRole detects whether a GlobalRole has admin permissions or not.
func (c *grbHandler) isAdminRole(rtName string) (bool, error) {
	gr, err := c.grLister.Get("", rtName)
	if err != nil {
		return false, err
	}

	// global role is builtin admin role
	if gr.Builtin && gr.Name == rbac.GlobalAdmin {
		return true, nil
	}

	var hasResourceRule, hasNonResourceRule bool
	for _, rule := range gr.Rules {
		if slice.ContainsString(rule.Resources, "*") && slice.ContainsString(rule.APIGroups, "*") && slice.ContainsString(rule.Verbs, "*") {
			hasResourceRule = true
			continue
		}
		if slice.ContainsString(rule.NonResourceURLs, "*") && slice.ContainsString(rule.Verbs, "*") {
			hasNonResourceRule = true
			continue
		}
	}

	// global role has an admin resource rule, and admin nonResourceURLs rule
	if hasResourceRule && hasNonResourceRule {
		return true, nil
	}

	return false, nil
}

func grbByUserAndRole(obj interface{}) ([]string, error) {
	grb, ok := obj.(*apisv3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{rbac.GetGRBTargetKey(grb) + "-" + grb.GlobalRoleName}, nil
}

// newCRTBtoGRBResolver returns a resolver which provides the key to the GRB that owns a given CRTB if one exist.
func newCRTBtoGRBResolver(grbCache mgmtv3.GlobalRoleBindingCache) relatedresource.Resolver {
	return func(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
		crtb, ok := obj.(*apisv3.ClusterRoleTemplateBinding)
		if !ok || crtb == nil {
			return nil, nil
		}

		var grbOwner *metav1.OwnerReference
		for i := range crtb.OwnerReferences {
			ref := &crtb.OwnerReferences[i]
			if ref.Kind == "GlobalRoleBinding" && ref.APIVersion == apisv3.SchemeGroupVersion.Version {
				grbOwner = ref
				break
			}
		}

		if grbOwner == nil {
			// there are no owner references to GlobalRoleBindings
			return nil, nil
		}

		grb, err := grbCache.Get(grbOwner.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get owner reference '%s' from cache: %w", grbOwner.Name, err)
		}

		return []relatedresource.Key{{Name: grb.Name}}, nil
	}
}
