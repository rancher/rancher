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
)

const (
	SummaryInProgress  = "InProgress"
	SummaryCompleted   = "Completed"
	SummaryError       = "Error"
	SummaryTerminating = "Terminating"
)

// Condition reason types
const (
	// BindingsExist is a success indicator. The CRTB-related bindings are all present and correct.
	BindingsExist = "BindingsExist"
	// CRTBExists
	CRTBExists = "CRTBExists"
	// FailedToDeleteClusterRoleBindings indicates that the controller was unable to delete the CRTB-related cluster role bindings.
	FailedToDeleteClusterRoleBindings = "FailedToDeleteClusterRoleBindings"
	// FailedToDeleteSAImpersonator indicates that the controller was unable to delete the impersonation account for the CRTB's user.
	FailedToDeleteSAImpersonator = "FailedToDeleteSAImpersonator"
	// FailedToEnsureClusterRoleBindings indicates that the controller was unable to create the cluster roles for the role template referenced by the CRTB.
	FailedToEnsureClusterRoleBindings = "FailedToEnsureClusterRoleBindings"
	// FailedToEnsureRoles indicates that the controller was unable to create the roles for the role template referenced by the CRTB.
	FailedToEnsureRoles = "FailedToEnsureRoles"
	// FailedToEnsureSAImpersonator means that the controller was unable to create the impersonation account for the CRTB's user.
	FailedToEnsureSAImpersonator = "FailedToEnsureSAImpersonator"
	// FailedToGetClusterRoleBindings means that the controller was unable to retrieve the CRTB-related cluster role bindings to update.
	FailedToGetClusterRoleBindings = "FailedToGetClusterRoleBindings"
	// FailedToGetLabelRequirements indicates issues with the CRTB meta data preventing creation of label requirements.
	FailedToGetLabelRequirements = "FailedToGetLabelRequirements"
	// FailedToGetRoleTemplate means that the controller failed to locate the role template referenced by the CRTB.
	FailedToGetRoleTemplate = "FailedToGetRoleTemplate"
	// FailedToGetRoles indicates that the controller failed to locate the roles for the role template referenced by the CRTB.
	FailedToGetRoles = "FailedToGetRoles"
	// FailedToUpdateCRTBLabels means the controller failed to update the CRTB labels indicating success of CRB/RB label updates.
	FailedToUpdateCRTBLabels = "FailedToUpdateCRTBLabels"
	// FailedToUpdateClusterRoleBindings means that the controller was unable to properly update the CRTB-related cluster role bindings.
	FailedToUpdateClusterRoleBindings = "FailedToUpdateClusterRoleBindings"
	// LabelsSet is a success indicator. The CRTB-related labels are all set.
	LabelsSet = "LabelsSet"
	// NoBindingsRequired is a success indicator.
	NoBindingsRequired = "NoBindingsRequired"
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
	if obj.Status.ObservedGeneration == obj.ObjectMeta.Generation &&
		obj.Status.Summary != SummaryInProgress {
		return obj, nil
	}
	returnError := errors.Join(
		c.setCRTBAsInProgress(obj),
		c.syncCRTB(obj),
		c.setCRTBAsCompleted(obj),
	)
	return obj, returnError
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	// ObjectMeta.Generation does not get updated when the Status is updated.
	// If only the status has been updated and we have finished updating the status
	// (status.Summary != "InProgress") we don't need to perform a reconcile as nothing has
	// changed.
	if obj.Status.ObservedGeneration == obj.ObjectMeta.Generation &&
		obj.Status.Summary != SummaryInProgress {
		return obj, nil
	}
	returnError := errors.Join(
		c.setCRTBAsInProgress(obj),
		c.reconcileCRTBUserClusterLabels(obj),
		c.syncCRTB(obj),
		c.setCRTBAsCompleted(obj),
	)
	return obj, returnError
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	returnError := errors.Join(
		c.setCRTBAsTerminating(obj),
		c.ensureCRTBDelete(obj),
	)
	return obj, returnError
}

func (c *crtbLifecycle) syncCRTB(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: BindingsExist}

	if binding.RoleTemplateName == "" {
		logrus.Warnf("ClusterRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		addCondition(binding, condition, NoBindingsRequired, binding.Name, nil)
		return nil
	}

	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		addCondition(binding, condition, NoBindingsRequired, binding.Name, nil)
		return nil
	}

	rt, err := c.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		err := fmt.Errorf("couldn't get role template %v: %w", binding.RoleTemplateName, err)
		addCondition(binding, condition, FailedToGetRoleTemplate, binding.Name, err)
		return err
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(rt, roles, 0); err != nil {
		addCondition(binding, condition, FailedToGetRoles, binding.Name, err)
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		err := fmt.Errorf("couldn't ensure roles: %w", err)
		addCondition(binding, condition, FailedToEnsureRoles, binding.Name, err)
		return err
	}

	if err := c.m.ensureClusterBindings(roles, binding); err != nil {
		err := fmt.Errorf("couldn't ensure cluster bindings %v: %w", binding, err)
		addCondition(binding, condition, FailedToEnsureClusterRoleBindings, binding.Name, err)
		return err
	}

	if binding.UserName != "" {
		if err := c.m.ensureServiceAccountImpersonator(binding.UserName); err != nil {
			err := fmt.Errorf("couldn't ensure service account impersonator: %w", err)
			addCondition(binding, condition, FailedToEnsureSAImpersonator, binding.UserName, err)
			return err
		}
	}

	addCondition(binding, condition, BindingsExist, binding.Name, nil)
	return nil
}

func (c *crtbLifecycle) ensureCRTBDelete(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: CRTBExists}

	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	rbs, err := c.crbLister.List("", set.AsSelector())
	if err != nil {
		addCondition(binding, condition, FailedToGetClusterRoleBindings, binding.UserName, nil)
		return fmt.Errorf("couldn't list clusterrolebindings with selector %s: %w", set.AsSelector(), err)
	}

	for _, rb := range rbs {
		if err := c.crbClient.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				addCondition(binding, condition, FailedToDeleteClusterRoleBindings, binding.UserName, nil)
				return fmt.Errorf("error deleting clusterrolebinding %v: %w", rb.Name, err)
			}
		}
	}

	if err := c.m.deleteServiceAccountImpersonator(binding.UserName); err != nil {
		addCondition(binding, condition, FailedToDeleteSAImpersonator, binding.UserName, nil)
		return fmt.Errorf("error deleting service account impersonator: %w", err)
	}

	return nil
}

func (c *crtbLifecycle) reconcileCRTBUserClusterLabels(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: LabelsSet}

	/* Prior to 2.5, for every CRTB, following CRBs are created in the user clusters
		1. CRTB.UID is the label value for a CRB, authz.cluster.cattle.io/rtb-owner=CRTB.UID
	Using this labels, list the CRBs and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[rtbCrbRbLabelsUpdated] == "true" {
		addCondition(binding, condition, LabelsSet, binding.Name, nil)
		return nil
	}

	var returnErr error
	set := labels.Set(map[string]string{rtbOwnerLabelLegacy: string(binding.UID)})
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		addCondition(binding, condition, FailedToGetLabelRequirements, binding.Name, err)
		return err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		addCondition(binding, condition, FailedToGetLabelRequirements, binding.Name, err)
		return err
	}
	set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel)
	userCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		addCondition(binding, condition, FailedToGetClusterRoleBindings, binding.Name, err)
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
		if retryErr != nil {
			addCondition(binding, condition, FailedToUpdateClusterRoleBindings, binding.Name, retryErr)
			returnErr = errors.Join(returnErr, retryErr)
		}
	}
	if returnErr != nil {
		// No condition here, already collected in the retries
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
		addCondition(binding, condition, FailedToUpdateCRTBLabels, binding.Name, retryErr)
		return retryErr
	}

	addCondition(binding, condition, LabelsSet, binding.Name, nil)
	return nil
}

func (c *crtbLifecycle) setCRTBAsInProgress(binding *v3.ClusterRoleTemplateBinding) error {
	binding.Status.Conditions = []metav1.Condition{}
	binding.Status.Summary = SummaryInProgress
	binding.Status.LastUpdate = time.Now().String()
	updatedCRTB, err := c.crtbClientM.UpdateStatus(binding)
	// For future updates, we want the latest version of our CRTB
	*binding = *updatedCRTB
	return err
}

func (c *crtbLifecycle) setCRTBAsCompleted(binding *v3.ClusterRoleTemplateBinding) error {
	binding.Status.Summary = SummaryCompleted
	for _, c := range binding.Status.Conditions {
		if c.Status != metav1.ConditionTrue {
			binding.Status.Summary = SummaryError
			break
		}
	}
	binding.Status.LastUpdate = time.Now().String()
	binding.Status.ObservedGeneration = binding.ObjectMeta.Generation
	updatedCRTB, err := c.crtbClientM.UpdateStatus(binding)
	// For future updates, we want the latest version of our CRTB
	*binding = *updatedCRTB
	return err
}

func (c *crtbLifecycle) setCRTBAsTerminating(binding *v3.ClusterRoleTemplateBinding) error {
	binding.Status.Conditions = []metav1.Condition{}
	binding.Status.Summary = SummaryTerminating
	binding.Status.LastUpdate = time.Now().String()
	_, err := c.crtbClientM.UpdateStatus(binding)
	return err
}

func addCondition(binding *v3.ClusterRoleTemplateBinding, condition metav1.Condition,
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
