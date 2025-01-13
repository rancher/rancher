package roletemplates

import (
	"errors"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/status"
	controllersv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type crtbHandler struct {
	impersonationHandler *impersonationHandler
	crbClient            wrbacv1.ClusterRoleBindingController
	crtbCache            controllersv3.ClusterRoleTemplateBindingCache
	crtbClient           controllersv3.ClusterRoleTemplateBindingClient
	s                    *status.Status
}

func newCRTBHandler(uc *config.UserContext) *crtbHandler {
	return &crtbHandler{
		impersonationHandler: &impersonationHandler{
			userContext: uc,
			crClient:    uc.RBACw.ClusterRole(),
			crtbClient:  uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
			crtbCache:   uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
			prtbClient:  uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding(),
			prtbCache:   uc.Management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		},
		crbClient:  uc.RBACw.ClusterRoleBinding(),
		crtbCache:  uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		crtbClient: uc.Management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		s:          status.NewStatus(),
	}
}

// OnChange ensures that the correct ClusterRoleBinding exists for the ClusterRoleTemplateBinding
func (c *crtbHandler) OnChange(key string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	remoteConditions := []metav1.Condition{}
	if crtb == nil || crtb.DeletionTimestamp != nil {
		return nil, nil
	}

	if err := c.reconcileBindings(crtb, &remoteConditions); err != nil {
		return nil, errors.Join(err, c.updateStatus(crtb, remoteConditions))
	}

	// Ensure a service account impersonator exists on the cluster
	var err error
	if crtb.UserName != "" {
		err = c.impersonationHandler.ensureServiceAccountImpersonator(crtb.UserName)
		c.s.AddCondition(&remoteConditions, metav1.Condition{Type: ensureServiceAccountImpersonator}, serviceAccountImpersonatorExists, err)
	}

	return crtb, errors.Join(err, c.updateStatus(crtb, remoteConditions))
}

// reconcileBindings builds and creates ClusterRoleBinding for CRTB and removes any CRBs that shouldn't exist
func (c *crtbHandler) reconcileBindings(crtb *v3.ClusterRoleTemplateBinding, remoteConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: reconcileClusterRoleBindings}

	crb, err := rbac.BuildClusterRoleBindingFromRTB(crtb, crtb.RoleTemplateName)
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failureToBuildClusterRoleBinding, err)
		return err
	}

	currentCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: rbac.GetCRTBOwnerLabel(crtb.Name)})
	if err != nil || currentCRBs == nil {
		c.s.AddCondition(remoteConditions, condition, failureToListClusterRoleBindings, err)
		return err
	}

	// Find if there is a CRB that already exists and delete all excess CRBs
	var matchingCRB *rbacv1.ClusterRoleBinding
	for _, currentCRB := range currentCRBs.Items {
		if rbac.AreClusterRoleBindingContentsSame(crb, &currentCRB) && matchingCRB == nil {
			matchingCRB = &currentCRB
			continue
		}
		if err := c.crbClient.Delete(currentCRB.Name, &metav1.DeleteOptions{}); err != nil {
			c.s.AddCondition(remoteConditions, condition, failureToDeleteClusterRoleBinding, err)
			return err
		}
	}

	// If we didn't find an existing CRB, create it
	if matchingCRB == nil {
		if _, err := c.crbClient.Create(crb); err != nil {
			c.s.AddCondition(remoteConditions, condition, failureToCreateClusterRoleBinding, err)
			return err
		}
	}
	c.s.AddCondition(remoteConditions, condition, clusterRoleBindingExists, nil)
	return nil
}

// OnRemove deletes all ClusterRoleBindings owned by the ClusterRoleTemplateBinding
func (c *crtbHandler) OnRemove(_ string, crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	err := c.deleteBindings(crtb, &crtb.Status.RemoteConditions)
	if err != nil {
		return crtb, errors.Join(err, c.updateStatus(crtb, crtb.Status.RemoteConditions))
	}

	if crtb.UserName != "" {
		err = c.impersonationHandler.deleteServiceAccountImpersonator(crtb.UserName)
		c.s.AddCondition(&crtb.Status.RemoteConditions, metav1.Condition{Type: deleteServiceAccountImpersonator}, failureToDeleteServiceAccount, err)
	}
	return nil, errors.Join(err, c.updateStatus(crtb, crtb.Status.RemoteConditions))
}

// deleteBindings removes cluster role bindings owned by CRTB
func (c *crtbHandler) deleteBindings(crtb *v3.ClusterRoleTemplateBinding, remoteConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: deleteClusterRoleBindings}

	lo := metav1.ListOptions{LabelSelector: rbac.GetCRTBOwnerLabel(crtb.Name)}

	crbs, err := c.crbClient.List(lo)
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failureToListClusterRoleBindings, err)
		return err
	}

	var returnError error
	for _, crb := range crbs.Items {
		err = c.crbClient.Delete(crb.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			returnError = errors.Join(returnError, err)
		}
	}

	c.s.AddCondition(remoteConditions, condition, clusterRoleBindingsDeleted, returnError)
	return returnError
}

var timeNow = func() time.Time {
	return time.Now()
}

func (c *crtbHandler) updateStatus(crtb *v3.ClusterRoleTemplateBinding, remoteConditions []metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbFromCluster, err := c.crtbCache.Get(crtb.Namespace, crtb.Name)
		if err != nil {
			return err
		}
		if status.CompareConditions(crtbFromCluster.Status.RemoteConditions, remoteConditions) {
			return nil
		}

		crtbFromCluster.Status.SummaryRemote = status.SummaryCompleted
		if crtbFromCluster.Status.SummaryLocal == status.SummaryCompleted {
			crtbFromCluster.Status.Summary = status.SummaryCompleted
		}
		for _, c := range remoteConditions {
			if c.Status != metav1.ConditionTrue {
				crtbFromCluster.Status.Summary = status.SummaryError
				crtbFromCluster.Status.SummaryRemote = status.SummaryError
				break
			}
		}

		crtbFromCluster.Status.LastUpdateTime = timeNow().Format(time.RFC3339)
		crtbFromCluster.Status.ObservedGenerationRemote = crtb.ObjectMeta.Generation
		crtbFromCluster.Status.RemoteConditions = remoteConditions
		_, err = c.crtbClient.UpdateStatus(crtbFromCluster)
		if err != nil {
			return err
		}
		return nil
	})
}
