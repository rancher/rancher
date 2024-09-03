package rbac

import (
	"errors"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/retry"

	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/controllers/status/crtb"
)

func newCRTBLifecycle(m *manager, management *config.ManagementContext) *crtbLifecycle {
	return &crtbLifecycle{
		m:           m,
		rtLister:    management.Management.RoleTemplates("").Controller().Lister(),
		crbLister:   m.workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crbClient:   m.workload.RBAC.ClusterRoleBindings(""),
		crtbClient:  management.Management.ClusterRoleTemplateBindings(""),
		crtbClientM: management.WithAgent("rbac-handler-base").Wrangler.Mgmt.ClusterRoleTemplateBinding(),
	}
}

// Local interface abstracting the mgmtconv3.ClusterRoleTemplateBindingController down to
// necessities. The testsuite then provides a local mock implementation for itself.
type clusterRoleTemplateBindingController interface {
	UpdateStatus(*v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error)
}

type crtbLifecycle struct {
	m           managerInterface
	rtLister    v3.RoleTemplateLister
	crbLister   typesrbacv1.ClusterRoleBindingLister
	crbClient   typesrbacv1.ClusterRoleBindingInterface
	crtbClient  v3.ClusterRoleTemplateBindingInterface
	crtbClientM clusterRoleTemplateBindingController
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status
	// (status.Summary != "InProgress") we don't need to perform a reconcile as nothing has
	// changed.
	if (obj.Status.ObservedGeneration == obj.ObjectMeta.Generation &&
		obj.Status.Summary != status.SummaryInProgress) ||
		status.HasAllOf(obj.Status.Conditions, crtb.RemoteSuccesses) {
		return obj, nil
	}
	if err := c.setCRTBAsInProgress(obj); err != nil {
		return obj, err
	}
	if err := c.syncCRTB(obj); err != nil {
		return obj, err
	}
	err := c.setCRTBAsCompleted(obj)
	return obj, err
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status
	// (status.Summary != "InProgress") we don't need to perform a reconcile as nothing has
	// changed.
	if (obj.Status.ObservedGeneration == obj.ObjectMeta.Generation &&
		obj.Status.Summary != status.SummaryInProgress) ||
		status.HasAllOf(obj.Status.Conditions, crtb.RemoteSuccesses) {
		return obj, nil
	}
	if err := c.setCRTBAsInProgress(obj); err != nil {
		return obj, err
	}
	if err := c.reconcileCRTBUserClusterLabels(obj); err != nil {
		return obj, err
	}
	if err := c.syncCRTB(obj); err != nil {
		return obj, err
	}
	err := c.setCRTBAsCompleted(obj)
	return obj, err
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	if err := c.setCRTBAsTerminating(obj); err != nil {
		return obj, err
	}
	err := c.ensureCRTBDelete(obj)
	return obj, err
}

func (c *crtbLifecycle) syncCRTB(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: crtb.RemoteBindingsExist}

	if binding.RoleTemplateName == "" {
		logrus.Warnf("ClusterRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		addCondition(binding, condition, crtb.RemoteBindingsExist, binding.Name, nil)
		return nil
	}

	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		addCondition(binding, condition, crtb.RemoteBindingsExist, binding.Name, nil)
		return nil
	}

	rt, err := c.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		addCondition(binding, condition, crtb.FailedToGetRoleTemplate, binding.Name, err)
		return fmt.Errorf("couldn't get role template %v: %w", binding.RoleTemplateName, err)
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(rt, roles, 0); err != nil {
		addCondition(binding, condition, crtb.FailedToGetRoles, binding.Name, err)
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		addCondition(binding, condition, crtb.FailedToEnsureRoles, binding.Name, err)
		return fmt.Errorf("couldn't ensure roles: %w", err)
	}

	if err := c.m.ensureClusterBindings(roles, binding); err != nil {
		addCondition(binding, condition, crtb.FailedToEnsureClusterRoleBindings, binding.Name, err)
		return fmt.Errorf("couldn't ensure cluster bindings %v: %w", binding, err)
	}

	if binding.UserName != "" {
		if err := c.m.ensureServiceAccountImpersonator(binding.UserName); err != nil {
			addCondition(binding, condition, crtb.FailedToEnsureSAImpersonator, binding.UserName, err)
			return fmt.Errorf("couldn't ensure service account impersonator: %w", err)
		}
	}

	addCondition(binding, condition, crtb.RemoteBindingsExist, binding.Name, nil)
	return nil
}

func (c *crtbLifecycle) ensureCRTBDelete(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: crtb.RemoteCRTBDeleteOk}

	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	rbs, err := c.crbLister.List("", set.AsSelector())
	if err != nil {
		addCondition(binding, condition, crtb.RemoteFailedToGetClusterRoleBindings, binding.UserName, err)
		return fmt.Errorf("couldn't list clusterrolebindings with selector %s: %w", set.AsSelector(), err)
	}

	for _, rb := range rbs {
		if err := c.crbClient.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				addCondition(binding, condition, crtb.FailedToDeleteClusterRoleBindings, binding.UserName, err)
				return fmt.Errorf("error deleting clusterrolebinding %v: %w", rb.Name, err)
			}
		}
	}

	if err := c.m.deleteServiceAccountImpersonator(binding.UserName); err != nil {
		addCondition(binding, condition, crtb.FailedToDeleteSAImpersonator, binding.UserName, err)
		return fmt.Errorf("error deleting service account impersonator: %w", err)
	}

	addCondition(binding, condition, crtb.RemoteCRTBDeleteOk, binding.UserName, nil)
	return nil
}

func (c *crtbLifecycle) reconcileCRTBUserClusterLabels(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: crtb.RemoteLabelsSet}

	/* Prior to 2.5, for every CRTB, following CRBs are created in the user clusters
		1. CRTB.UID is the label value for a CRB, authz.cluster.cattle.io/rtb-owner=CRTB.UID
	Using this labels, list the CRBs and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[rtbCrbRbLabelsUpdated] == "true" {
		addCondition(binding, condition, crtb.RemoteLabelsSet, binding.Name, nil)
		return nil
	}

	var returnErr error
	set := labels.Set(map[string]string{rtbOwnerLabelLegacy: string(binding.UID)})
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		addCondition(binding, condition, crtb.RemoteFailedToGetLabelRequirements, binding.Name, err)
		return err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		addCondition(binding, condition, crtb.RemoteFailedToGetLabelRequirements, binding.Name, err)
		return err
	}
	set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel)
	userCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		addCondition(binding, condition, crtb.RemoteFailedToGetClusterRoleBindings, binding.Name, err)
		return err
	}
	bindingValue := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, crb := range userCRBs.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := c.crbClient.Get(crb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[rtbOwnerLabel] = bindingValue
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := c.crbClient.Update(crbToUpdate)
			return err
		})
		returnErr = errors.Join(returnErr, retryErr)
	}
	if returnErr != nil {
		addCondition(binding, condition, crtb.RemoteFailedToUpdateClusterRoleBindings, binding.Name, returnErr)
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbToUpdate, updateErr := c.crtbClient.GetNamespaced(binding.Namespace, binding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		if crtbToUpdate.Labels == nil {
			crtbToUpdate.Labels = make(map[string]string)
		}
		crtbToUpdate.Labels[rtbCrbRbLabelsUpdated] = "true"
		_, err := c.crtbClient.Update(crtbToUpdate)
		return err
	})

	if retryErr != nil {
		addCondition(binding, condition, crtb.RemoteFailedToUpdateCRTBLabels, binding.Name, retryErr)
		return retryErr
	}

	addCondition(binding, condition, crtb.RemoteLabelsSet, binding.Name, nil)
	return nil
}

// Status field management, condition management

func (c *crtbLifecycle) setCRTBAsInProgress(binding *v3.ClusterRoleTemplateBinding) error {
	// Keep information managed by the local controller.
	// Wipe only information managed here
	binding.Status.Conditions = status.RemoveConditions(binding.Status.Conditions, crtb.RemoteConditions)

	binding.Status.Summary = status.SummaryInProgress
	binding.Status.LastUpdateTime = time.Now().String()
	updatedCRTB, err := c.crtbClientM.UpdateStatus(binding)
	if err != nil {
		return err
	}
	// For future updates, we want the latest version of our CRTB
	*binding = *updatedCRTB
	return nil
}

func (c *crtbLifecycle) setCRTBAsCompleted(binding *v3.ClusterRoleTemplateBinding) error {
	// set summary based on error conditions
	failed := false
	for _, c := range binding.Status.Conditions {
		if c.Status != metav1.ConditionTrue {
			binding.Status.Summary = status.SummaryError
			failed = true
			break
		}
	}

	// no error conditions. check for all (local and remote!) success conditions
	// note: keep the status as in progress if only partial sucess was found
	if !failed && status.HasAllOf(binding.Status.Conditions, crtb.Successes) {
		binding.Status.Summary = status.SummaryCompleted
	}

	binding.Status.LastUpdateTime = time.Now().String()
	binding.Status.ObservedGeneration = binding.ObjectMeta.Generation
	updatedCRTB, err := c.crtbClientM.UpdateStatus(binding)
	if err != nil {
		return err
	}
	// For future updates, we want the latest version of our CRTB
	*binding = *updatedCRTB
	return nil
}

func (c *crtbLifecycle) setCRTBAsTerminating(binding *v3.ClusterRoleTemplateBinding) error {
	binding.Status.Conditions = []metav1.Condition{}
	binding.Status.Summary = status.SummaryTerminating
	binding.Status.LastUpdateTime = time.Now().String()
	_, err := c.crtbClientM.UpdateStatus(binding)
	return err
}

func addCondition(binding *v3.ClusterRoleTemplateBinding, condition metav1.Condition, reason, name string, err error) {
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
