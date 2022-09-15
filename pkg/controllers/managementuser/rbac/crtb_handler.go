package rbac

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/retry"
)

func newCRTBLifecycle(m *manager) *crtbLifecycle {
	return &crtbLifecycle{m: m}
}

type crtbLifecycle struct {
	m *manager
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

	rt, err := c.m.rtLister.Get("", binding.RoleTemplateName)
	if err != nil {
		return errors.Wrapf(err, "couldn't get role template %v", binding.RoleTemplateName)
	}

	roles := map[string]*v3.RoleTemplate{}
	if err := c.m.gatherRoles(rt, roles, 0); err != nil {
		return err
	}

	if err := c.m.ensureRoles(roles); err != nil {
		return errors.Wrap(err, "couldn't ensure roles")
	}

	if err := c.m.ensureClusterBindings(roles, binding); err != nil {
		return errors.Wrapf(err, "couldn't ensure cluster bindings %v", binding)
	}

	if binding.UserName != "" {
		if err := c.m.ensureServiceAccountImpersonator(binding.UserName); err != nil {
			return errors.Wrapf(err, "couldn't ensure service account impersonator")
		}
	}

	return nil
}

func (c *crtbLifecycle) ensureCRTBDelete(binding *v3.ClusterRoleTemplateBinding) error {
	set := labels.Set(map[string]string{rtbOwnerLabel: pkgrbac.GetRTBLabel(binding.ObjectMeta)})
	bindingCli := c.m.workload.RBAC.ClusterRoleBindings("")
	rbs, err := c.m.crbLister.List("", set.AsSelector())
	if err != nil {
		return errors.Wrapf(err, "couldn't list clusterrolebindings with selector %s", set.AsSelector())
	}

	for _, rb := range rbs {
		if err := bindingCli.Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "error deleting clusterrolebinding %v", rb.Name)
			}
		}
	}

	if err := c.m.deleteServiceAccountImpersonator(binding.UserName); err != nil {
		return errors.Wrap(err, "error deleting service account impersonator")
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
	userCRBs, err := c.m.clusterRoleBindings.List(metav1.ListOptions{LabelSelector: set.AsSelector().Add(*reqUpdatedLabel, *reqNsAndNameLabel).String()})
	if err != nil {
		return err
	}
	bindingValue := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, crb := range userCRBs.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := c.m.clusterRoleBindings.Get(crb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[rtbOwnerLabel] = bindingValue
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := c.m.clusterRoleBindings.Update(crbToUpdate)
			return err
		})
		if retryErr != nil {
			returnErr = multierror.Append(returnErr, retryErr)
		}
	}
	if returnErr != nil {
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbToUpdate, updateErr := c.m.crtbs.GetNamespaced(binding.Namespace, binding.Name, metav1.GetOptions{})
		if updateErr != nil {
			return updateErr
		}
		if crtbToUpdate.Labels == nil {
			crtbToUpdate.Labels = make(map[string]string)
		}
		crtbToUpdate.Labels[rtbCrbRbLabelsUpdated] = "true"
		_, err := c.m.crtbs.Update(crtbToUpdate)
		return err
	})
	return retryErr
}
