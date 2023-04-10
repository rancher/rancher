package rbac

import (
	"fmt"

	"github.com/rancher/norman/types/slice"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	v12 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	grbByUserAndRoleIndex = "authz.cluster.cattle.io/grb-by-user-and-role"
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
	informer := workload.Management.Management.GlobalRoleBindings("").Controller().Informer()

	h := &grbHandler{
		clusterName:         workload.ClusterName,
		grbIndexer:          informer.GetIndexer(),
		clusterRoleBindings: workload.RBAC.ClusterRoleBindings(""),
		crbLister:           workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		// The following clients/controllers all point at the management cluster
		crtbLister:                  workload.Management.Management.ClusterRoleTemplateBindings("").Controller().Lister(),
		clusterRoleTemplateBindings: workload.Management.Management.ClusterRoleTemplateBindings(""),
		grLister:                    workload.Management.Management.GlobalRoles("").Controller().Lister(),
		rbLister:                    workload.Management.RBAC.RoleBindings("").Controller().Lister(),
		roleBindings:                workload.Management.RBAC.RoleBindings(""),
		globalroleBindingController: workload.Management.Management.GlobalRoleBindings("").Controller(),
		clusters:                    workload.Management.Management.Clusters(""),
		provClusters:                workload.Management.Wrangler.Provisioning.Cluster().Cache(),
	}

	return h.sync
}

// grbHandler ensures the global admins have full access to every cluster. If a globalRoleBinding is created that uses
// the admin role, then the user in that binding gets a clusterRoleBinding in every user cluster to the cluster-admin role
type grbHandler struct {
	clusterName                 string
	clusterRoleBindings         rbacv1.ClusterRoleBindingInterface
	crbLister                   rbacv1.ClusterRoleBindingLister
	grbIndexer                  cache.Indexer
	crtbLister                  v3.ClusterRoleTemplateBindingLister
	clusterRoleTemplateBindings v3.ClusterRoleTemplateBindingInterface
	grLister                    v3.GlobalRoleLister
	rbLister                    rbacv1.RoleBindingLister
	roleBindings                rbacv1.RoleBindingInterface
	globalroleBindingController v3.GlobalRoleBindingController
	clusters                    v3.ClusterInterface
	provClusters                provisioningcontrollers.ClusterCache
}

func (c *grbHandler) sync(key string, obj *v3.GlobalRoleBinding) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	isAdmin, err := c.isAdminRole(obj.GlobalRoleName)
	if err != nil {
		return nil, err
	} else if !isAdmin {
		return obj, nil
	}

	// Do not sync restricted-admin to the local cluster as 'cluster-admin'
	if c.clusterName == "local" && obj.GlobalRoleName == rbac.GlobalRestrictedAdmin {
		return obj, nil
	}

	logrus.Debugf("%v is an admin role", obj.GlobalRoleName)

	err = c.addAdminAsClusterAdmin(obj)
	if err != nil {
		return nil, err
	}

	err = c.addRestrictedAdminCRTB(obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// addAdminAsClusterAdmin creates a cluster role binding of an admin user
// to the regular Kubernetes "cluster-admin" cluster role in the downstream cluster.
func (c *grbHandler) addAdminAsClusterAdmin(obj *v3.GlobalRoleBinding) error {
	if obj == nil || obj.GlobalRoleName != rbac.GlobalAdmin {
		return nil
	}
	bindingName := rbac.GrbCRBName(obj)
	b, err := c.crbLister.Get("", bindingName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if b != nil {
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
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

// addRestrictedAdminCRTB adds a restricted-admin to the downstream cluster as cluster-owner
// by creating a CRTB with the "cluster-owner" role template.
func (c *grbHandler) addRestrictedAdminCRTB(obj *v3.GlobalRoleBinding) error {
	// Restricted-admin needs this, a regular admin will already have access to all the resources
	// this binding grants in the management cluster.
	if obj.GlobalRoleName != rbac.GlobalRestrictedAdmin {
		return nil
	}

	crtbName := name.SafeConcatName(rbac.GetGRBTargetKey(obj), "restricted-admin", "cluster-owner")
	_, err := c.crtbLister.Get(c.clusterName, crtbName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get CRTB from cache: %w", err)
	}
	if err == nil {
		// CRTB was already created.
		return nil
	}

	// Add the restricted admin user as a member of the downstream cluster
	// by creating a CRTB in the local custer in the namespace named after the downstream cluster.
	crtb := v3.ClusterRoleTemplateBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crtbName,
			Namespace: c.clusterName,
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
	if obj.UserName != "" {
		crtb.UserName = obj.UserName
	} else {
		crtb.GroupPrincipalName = obj.GroupPrincipalName
	}

	_, err = c.clusterRoleTemplateBindings.Create(&crtb)
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
	if gr.Builtin && (gr.Name == rbac.GlobalAdmin || gr.Name == rbac.GlobalRestrictedAdmin) {
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
	grb, ok := obj.(*v3.GlobalRoleBinding)
	if !ok {
		return []string{}, nil
	}

	return []string{rbac.GetGRBTargetKey(grb) + "-" + grb.GlobalRoleName}, nil
}
