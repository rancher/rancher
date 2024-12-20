package rbac

import (
	"errors"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/controllers/status"

	controllersv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
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
	clusterRolesExists                       = "ClusterRolesExists"
	clusterRoleBindingsExists                = "ClusterRoleBindingsExists"
	serviceAccountImpersonatorExists         = "ServiceAccountImpersonatorExists"
	crtbLabelsUpdated                        = "CRTBLabelsUpdated"
	clusterRoleTemplateBindingDelete         = "ClusterRoleTemplateBindingDelete"
	roleTemplateDoesNotExist                 = "RoleTemplateDoesNotExist"
	userOrGroupDoesNotExist                  = "UserOrGroupDoesNotExist"
	failedToGetRoleTemplate                  = "FailedToGetRoleTemplate"
	failedToGatherRoles                      = "FailedToGatherRoles"
	failedToCreateRoles                      = "FailedToCreateRoles"
	failedToCreateBindings                   = "FailedToCreateBindings"
	failedToCreateServiceAccountImpersonator = "FailedToCreateServiceAccountImpersonator"
	failedToCreateLabelRequirement           = "FailedToCreateLabelRequirement"
	failedToListCRBs                         = "FailedToListCRBs"
	failedToUpdateCRBs                       = "FailedToUpdateCRBs"
	failedToDeleteClusterRoleBinding         = "FailedToDeleteClusterRoleBinding"
	failedToDeleteServiceAccountImpersonator = "FailedToDeleteServiceAccountImpersonator"
)

func newCRTBLifecycle(m *manager, management *config.ManagementContext) *crtbLifecycle {
	return &crtbLifecycle{
		m:          m,
		rtLister:   management.Management.RoleTemplates("").Controller().Lister(),
		crbLister:  m.workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crbClient:  m.workload.RBAC.ClusterRoleBindings(""),
		crtbClient: management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		crtbCache:  management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		s:          status.NewStatus(),
	}
}

type crtbLifecycle struct {
	m          managerInterface
	rtLister   v3.RoleTemplateLister
	crbLister  typesrbacv1.ClusterRoleBindingLister
	crbClient  typesrbacv1.ClusterRoleBindingInterface
	crtbClient controllersv3.ClusterRoleTemplateBindingController
	crtbCache  controllersv3.ClusterRoleTemplateBindingCache
	s          *status.Status
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	remoteConditions := []metav1.Condition{}
	return obj, errors.Join(c.syncCRTB(obj, &remoteConditions),
		c.updateStatus(obj, remoteConditions))
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	remoteConditions := []metav1.Condition{}
	return obj, errors.Join(c.reconcileCRTBUserClusterLabels(obj, &remoteConditions),
		c.syncCRTB(obj, &remoteConditions),
		c.updateStatus(obj, remoteConditions))
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	err := c.ensureCRTBDelete(obj, &obj.Status.RemoteConditions)
	if err != nil {
		return obj, errors.Join(err,
			c.updateStatus(obj, obj.Status.RemoteConditions))
	}
	return obj, err
}

func (c *crtbLifecycle) syncCRTB(binding *v3.ClusterRoleTemplateBinding, remoteConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: clusterRolesExists}

	if binding.RoleTemplateName == "" {
		logrus.Warnf("ClusterRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		c.s.AddCondition(remoteConditions, condition, roleTemplateDoesNotExist, nil)
		return nil
	}

	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		c.s.AddCondition(remoteConditions, condition, userOrGroupDoesNotExist, nil)
		return nil
	}

	rt, err := c.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		err = fmt.Errorf("couldn't get role template %v: %w", binding.RoleTemplateName, err)
		c.s.AddCondition(remoteConditions, condition, failedToGetRoleTemplate, err)
		return err
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(rt, roles, 0); err != nil {
		c.s.AddCondition(remoteConditions, condition, failedToGatherRoles, err)
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		err = fmt.Errorf("couldn't ensure roles: %w", err)
		c.s.AddCondition(remoteConditions, condition, failedToCreateRoles, err)
		return err
	}
	c.s.AddCondition(remoteConditions, condition, clusterRolesExists, nil)

	condition = metav1.Condition{Type: clusterRoleBindingsExists}
	if err := c.m.ensureClusterBindings(roles, binding); err != nil {
		err = fmt.Errorf("couldn't ensure cluster bindings %v: %w", binding.Name, err)
		c.s.AddCondition(remoteConditions, condition, failedToCreateBindings, err)
		return err
	}
	c.s.AddCondition(remoteConditions, condition, clusterRoleBindingsExists, nil)

	condition = metav1.Condition{Type: serviceAccountImpersonatorExists}
	if binding.UserName != "" {
		if err := c.m.ensureServiceAccountImpersonator(binding.UserName); err != nil {
			err = fmt.Errorf("couldn't ensure service account impersonator: %w", err)
			c.s.AddCondition(remoteConditions, condition, failedToCreateServiceAccountImpersonator, err)
			return err
		}
	}
	c.s.AddCondition(remoteConditions, condition, serviceAccountImpersonatorExists, nil)

	return nil
}

func (c *crtbLifecycle) ensureCRTBDelete(binding *v3.ClusterRoleTemplateBinding, remoteConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: clusterRoleTemplateBindingDelete}

	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	rbs, err := c.crbLister.List("", set.AsSelector())
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failedToListCRBs, err)
		return fmt.Errorf("couldn't list clusterrolebindings with selector %s: %w", set.AsSelector(), err)
	}

	for _, rb := range rbs {
		if err := c.crbClient.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				c.s.AddCondition(remoteConditions, condition, failedToDeleteClusterRoleBinding, err)
				return fmt.Errorf("error deleting clusterrolebinding %v: %w", rb.Name, err)
			}
		}
	}

	if err := c.m.deleteServiceAccountImpersonator(binding.UserName); err != nil {
		c.s.AddCondition(remoteConditions, condition, failedToDeleteServiceAccountImpersonator, err)
		return fmt.Errorf("error deleting service account impersonator: %w", err)
	}

	return nil
}

func (c *crtbLifecycle) reconcileCRTBUserClusterLabels(binding *v3.ClusterRoleTemplateBinding, remoteConditions *[]metav1.Condition) error {
	/* Prior to 2.5, for every CRTB, following CRBs are created in the user clusters
		1. CRTB.UID is the label value for a CRB, authz.cluster.cattle.io/rtb-owner=CRTB.UID
	Using this labels, list the CRBs and update them to add a label with ns+name of CRTB
	*/
	condition := metav1.Condition{Type: crtbLabelsUpdated}

	if binding.Labels[rtbCrbRbLabelsUpdated] == "true" {
		c.s.AddCondition(remoteConditions, condition, crtbLabelsUpdated, nil)
		return nil
	}

	var returnErr error
	set := labels.Set(map[string]string{rtbOwnerLabelLegacy: string(binding.UID)})
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failedToCreateLabelRequirement, err)
		return err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failedToCreateLabelRequirement, err)
		return err
	}
	set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel)
	userCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		c.s.AddCondition(remoteConditions, condition, failedToListCRBs, err)
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
		c.s.AddCondition(remoteConditions, condition, failedToUpdateCRBs, returnErr)
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbToUpdate, updateErr := c.crtbClient.Get(binding.Namespace, binding.Name, metav1.GetOptions{})
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
		c.s.AddCondition(remoteConditions, condition, failedToUpdateCRBs, returnErr)
		return returnErr
	}

	c.s.AddCondition(remoteConditions, condition, crtbLabelsUpdated, nil)

	return nil
}

var timeNow = func() time.Time {
	return time.Now()
}

func (c *crtbLifecycle) updateStatus(crtb *v3.ClusterRoleTemplateBinding, remoteConditions []metav1.Condition) error {
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
		crtbFromCluster, err = c.crtbClient.UpdateStatus(crtbFromCluster)
		if err != nil {
			return err
		}
		return nil
	})
}
