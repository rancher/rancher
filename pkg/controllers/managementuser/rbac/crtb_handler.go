package rbac

import (
	"errors"
	"fmt"

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

func newCRTBLifecycle(m *manager, management *config.ManagementContext) *crtbLifecycle {
	return &crtbLifecycle{
		m:          m,
		rtLister:   management.Management.RoleTemplates("").Controller().Lister(),
		crbLister:  m.workload.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crbClient:  m.workload.RBAC.ClusterRoleBindings(""),
		crtbClient: management.Management.ClusterRoleTemplateBindings(""),
	}
}

type crtbLifecycle struct {
	m          managerInterface
	rtLister   v3.RoleTemplateLister
	crbLister  typesrbacv1.ClusterRoleBindingLister
	crbClient  typesrbacv1.ClusterRoleBindingInterface
	crtbClient v3.ClusterRoleTemplateBindingInterface
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	err := c.syncCRTB(obj)
	return obj, err
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	if err := c.reconcileCRTBUserClusterLabels(obj); err != nil {
		return obj, err
	}
	err := c.syncCRTB(obj)
	return obj, err
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	err := c.ensureCRTBDelete(obj)
	return obj, err
}

func (c *crtbLifecycle) syncCRTB(binding *v3.ClusterRoleTemplateBinding) error {
	if binding.RoleTemplateName == "" {
		logrus.Warnf("ClusterRoleTemplateBinding %v has no role template set. Skipping.", binding.Name)
		return nil
	}

	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		return nil
	}

	rt, err := c.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return fmt.Errorf("couldn't get role template %v: %w", binding.RoleTemplateName, err)
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(rt, roles, 0); err != nil {
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		return fmt.Errorf("couldn't ensure roles: %w", err)
	}

	if err := c.m.ensureClusterBindings(roles, binding); err != nil {
		return fmt.Errorf("couldn't ensure cluster bindings %v: %w", binding, err)
	}

	if binding.UserName != "" {
		if err := c.m.ensureServiceAccountImpersonator(binding.UserName); err != nil {
			return fmt.Errorf("couldn't ensure service account impersonator: %w", err)
		}
	}

	return nil
}

func (c *crtbLifecycle) ensureCRTBDelete(binding *v3.ClusterRoleTemplateBinding) error {
	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	rbs, err := c.crbLister.List("", set.AsSelector())
	if err != nil {
		return fmt.Errorf("couldn't list clusterrolebindings with selector %s: %w", set.AsSelector(), err)
	}

	for _, rb := range rbs {
		if err := c.crbClient.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("error deleting clusterrolebinding %v: %w", rb.Name, err)
			}
		}
	}

	if err := c.m.deleteServiceAccountImpersonator(binding.UserName); err != nil {
		return fmt.Errorf("error deleting service account impersonator: %w", err)
	}

	return nil
}

func (c *crtbLifecycle) reconcileCRTBUserClusterLabels(binding *v3.ClusterRoleTemplateBinding) error {
	/* Prior to 2.5, for every CRTB, following CRBs are created in the user clusters
		1. CRTB.UID is the label value for a CRB, authz.cluster.cattle.io/rtb-owner=CRTB.UID
	Using this labels, list the CRBs and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[rtbCrbRbLabelsUpdated] == "true" {
		return nil
	}

	var returnErr error
	set := labels.Set(map[string]string{rtbOwnerLabelLegacy: string(binding.UID)})
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(rtbOwnerLabel, selection.DoesNotExist, []string{})
	if err != nil {
		return err
	}
	set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel)
	userCRBs, err := c.crbClient.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
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
	return retryErr
}
