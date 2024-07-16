package rbac

import (
	"errors"
	"fmt"
	"time"

	"github.com/rancher/norman/types/slice"
	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
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
)

// Condition reason types
const (
	// CRBExists is a success indicator
	CRBExists = "CRBExists"
	// FailedToGetCRB indicates that the controller failed to retrieve an existing associated CRB
	FailedToGetCRB = "FailedToGetCRB"
	// FailedToCreateCRB indicates that the controller failed to create a missing associated CRB
	FailedToCreateCRB = "FailedToCreateCRB"
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
		grbClient:           workload.Management.WithAgent("rbac-handler-base").Wrangler.Mgmt.GlobalRoleBinding(),
		// The following clients/controllers all point at the management cluster
		grLister: workload.Management.Management.GlobalRoles("").Controller().Lister(),
	}

	return h.sync
}

// grbHandler ensures the global admins have full access to every cluster. If a globalRoleBinding is created that uses
// the admin role, then the user in that binding gets a clusterRoleBinding in every user cluster to the cluster-admin role
type grbHandler struct {
	clusterName         string
	clusterRoleBindings rbacv1.ClusterRoleBindingInterface
	crbLister           rbacv1.ClusterRoleBindingLister
	grLister            v3.GlobalRoleLister
	grbClient           globalRoleBindingController
}

// Local interface abstracting the mgmtconv3.GlobalRoleBindingController down to
// necessities. The testsuite then provides a local mock implementation for itself.
type globalRoleBindingController interface {
	UpdateStatus(*apisv3.GlobalRoleBinding) (*apisv3.GlobalRoleBinding, error)
}

func (c *grbHandler) sync(key string, obj *apisv3.GlobalRoleBinding) (runtime.Object, error) {
	if obj != nil {
		logrus.Debugf("GRB key %v `%v` deleted? %v", key, obj.GlobalRoleName, obj.DeletionTimestamp)
	} else {
		logrus.Debugf("GRB key %v, deleted?", key)
	}

	// ignore deleted resources (and those pending final removal)
	if obj == nil || obj.DeletionTimestamp != nil {
		if obj != nil {
			// Mark as in deletion
			return obj, c.setGRBAsTerminating(obj)
		}
		return obj, nil
	}

	// ignore non-admin roles
	isAdmin, err := c.isAdminRole(obj.GlobalRoleName)
	if err != nil {
		return nil, err
	}
	if !isAdmin {
		logrus.Debugf("GRB %v ignored, not an admin role", obj.GlobalRoleName)
		return obj, nil
	}

	logrus.Debugf("GRB %v is an admin role", obj.GlobalRoleName)

	// status for create|update of admin roles
	return obj, errors.Join(
		c.setGRBAsInProgress(obj),
		c.ensureClusterAdminBinding(obj),
		c.setGRBAsCompleted(obj),
	)
}

// ensureClusterAdminBinding creates a ClusterRoleBinding for GRB subject to
// the Kubernetes "cluster-admin" ClusterRole in the downstream cluster.
func (c *grbHandler) ensureClusterAdminBinding(obj *apisv3.GlobalRoleBinding) error {
	condition := metav1.Condition{Type: CRBExists}

	bindingName := rbac.GrbCRBName(obj)
	_, err := c.crbLister.Get("", bindingName)
	if err != nil && !apierrors.IsNotFound(err) {
		addGRBCondition(obj, condition, FailedToGetCRB, bindingName, err)
		return fmt.Errorf("failed to get ClusterRoleBinding '%s' from the cache: %w", bindingName, err)
	}

	if err == nil {
		// binding exists, nothing to do
		addGRBCondition(obj, condition, CRBExists, bindingName, nil)
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
		addGRBCondition(obj, condition, FailedToCreateCRB, bindingName, err)
		return fmt.Errorf("failed to create ClusterRoleBinding '%s' for admin in downstream '%s': %w", bindingName, c.clusterName, err)
	}

	addGRBCondition(obj, condition, CRBExists, bindingName, nil)
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

func (c *grbHandler) setGRBAsInProgress(binding *v3.GlobalRoleBinding) error {
	binding.Status.Conditions = []metav1.Condition{}
	binding.Status.Summary = SummaryInProgress
	binding.Status.LastUpdateTime = time.Now().String()
	updatedGRB, err := c.grbClient.UpdateStatus(binding)
	if err != nil {
		return err
	}
	// For future updates, we want the latest version of our GRB
	*binding = *updatedGRB
	return nil
}

func (c *grbHandler) setGRBAsCompleted(binding *v3.GlobalRoleBinding) error {
	binding.Status.Summary = SummaryCompleted
	for _, c := range binding.Status.Conditions {
		if c.Status != metav1.ConditionTrue {
			binding.Status.Summary = SummaryError
			break
		}
	}
	binding.Status.LastUpdateTime = time.Now().String()
	binding.Status.ObservedGeneration = binding.ObjectMeta.Generation
	updatedGRB, err := c.grbClient.UpdateStatus(binding)
	if err != nil {
		return err
	}
	// For future updates, we want the latest version of our GRB
	*binding = *updatedGRB
	return nil
}

func (c *grbHandler) setGRBAsTerminating(binding *v3.GlobalRoleBinding) error {
	binding.Status.Conditions = []metav1.Condition{}
	binding.Status.Summary = SummaryTerminating
	binding.Status.LastUpdateTime = time.Now().String()
	_, err := c.grbClient.UpdateStatus(binding)
	return err
}

func addGRBCondition(binding *v3.GlobalRoleBinding, condition metav1.Condition,
	reason, name string, err error) {
	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Message = fmt.Sprintf("%s not created: %v", name, err)
	} else {
		condition.Status = metav1.ConditionTrue
		condition.Message = fmt.Sprintf("%s created", name)
	}
	condition.Reason = reason
	condition.LastTransitionTime = metav1.Time{Time: time.Now()}
	binding.Status.Conditions = append(binding.Status.Conditions, condition)
}
