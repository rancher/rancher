package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/status"

	"github.com/rancher/rancher/pkg/controllers/management/authprovisioningv2"
	controllersv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	/* Prior to 2.5, the label "memberhsip-binding-owner" was set on the CRB/RBs for a roleTemplateBinding with the key being the roleTemplateBinding's UID.
	2.5 onwards, instead of the roleTemplateBinding's UID, a combination of its namespace and name will be used in this label.
	CRB/RBs on clusters upgraded from 2.4.x to 2.5 will continue to carry the original label with UID. To ensure permissions are managed properly on upgrade,
	we need to change the label value as well.
	So the older label value, MembershipBindingOwnerLegacy (<=2.4.x) will continue to be "memberhsip-binding-owner" (notice the spelling mistake),
	and the new label, MembershipBindingOwner will be "membership-binding-owner" (a different label value with the right spelling)*/
	MembershipBindingOwnerLegacy = "memberhsip-binding-owner"
	MembershipBindingOwner       = "membership-binding-owner"
	clusterResource              = "clusters"
	membershipBindingOwnerIndex  = "auth.management.cattle.io/membership-binding-owner"
	CrtbInProjectBindingOwner    = "crtb-in-project-binding-owner"
	PrtbInClusterBindingOwner    = "prtb-in-cluster-binding-owner"
	rbByOwnerIndex               = "auth.management.cattle.io/rb-by-owner"
	rbByRoleAndSubjectIndex      = "auth.management.cattle.io/crb-by-role-and-subject"
	ctrbMGMTController           = "mgmt-auth-crtb-controller"
	rtbLabelUpdated              = "auth.management.cattle.io/rtb-label-updated"
	RtbCrbRbLabelsUpdated        = "auth.management.cattle.io/crb-rb-labels-updated"

	subjectExists                                                    = "SubjectExists"
	bindingExists                                                    = "BindingExists"
	labelsReconciled                                                 = "LabelsReconciled"
	clusterRoleTemplateBindingDelete                                 = "ClusterRoleTemplateBindingDelete"
	failedToCreateUser                                               = "FailedToCreateUser"
	failedToGetUser                                                  = "FailedToGetUser"
	crtbHasNoSubject                                                 = "CRTBHasNoSubject"
	failedToGetCluster                                               = "FailedToGetCluster"
	clusterNotFound                                                  = "ClusterNotFound"
	failedToCheckReferencedRole                                      = "FailedToCheckReferencedRole"
	failedToBuildSubject                                             = "FailedToBuildSubject"
	failedToEnsureClusterMembershipBinding                           = "FailedToEnsureClusterMembershipBinding"
	failedToGrantManagementPlanePrivileges                           = "FailedToGrantManagementPlanePrivileges"
	failedToListProjects                                             = "FailedToListProjects"
	failedToGrantManagementClusterScopedPrivilegesInProjectNamespace = "FailedToGrantManagementClusterScopedPrivilegesInProjectNamespace"
	failedToGetRequirements                                          = "FailedToGetRequirements"
	failedToGetClusterRoleBindings                                   = "FailedToGetClusterRoleBindings"
	failedToUpdateClusterRoleBindings                                = "FailedToUpdateClusterRoleBindings"
	failedToListRB                                                   = "FailedToListRB"
	failedToUpdateClusterRoleTemplateBindings                        = "FailedToUpdateClusterRoleTemplateBindings"
	failedToDeleteClusterMembershipBinding                           = "FailedToDeleteClusterMembershipBinding"
	failedToDeleteMGMTClusterScopedPrivilegesInProjectNamespace      = "FailedToDeleteMGMTClusterScopedPrivilegesInProjectNamespace"
	failedToDeleteAuthV2Permissions                                  = "FailedToDeleteAuthV2Permissions"
)

var clusterManagementPlaneResources = map[string]string{
	"clusterscans":                "management.cattle.io",
	"clusterregistrationtokens":   "management.cattle.io",
	"clusterroletemplatebindings": "management.cattle.io",
	"etcdbackups":                 "management.cattle.io",
	"nodes":                       "management.cattle.io",
	"nodepools":                   "management.cattle.io",
	"projects":                    "management.cattle.io",
	"etcdsnapshots":               "rke.cattle.io",
}

type crtbLifecycle struct {
	mgr           managerInterface
	clusterLister v3.ClusterLister
	userMGR       user.Manager
	userLister    v3.UserLister
	projectLister v3.ProjectLister
	rbLister      typesrbacv1.RoleBindingLister
	rbClient      typesrbacv1.RoleBindingInterface
	crbLister     typesrbacv1.ClusterRoleBindingLister
	crbClient     typesrbacv1.ClusterRoleBindingInterface
	crtbClient    controllersv3.ClusterRoleTemplateBindingController
	crtbCache     controllersv3.ClusterRoleTemplateBindingCache
	s             *status.Status
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	var localConditions []metav1.Condition
	obj, err := c.reconcileSubject(obj, &localConditions)
	return obj, errors.Join(err,
		c.reconcileBindings(obj, &localConditions),
		c.updateStatus(obj, localConditions))
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	var localConditions []metav1.Condition
	obj, err := c.reconcileSubject(obj, &localConditions)
	return obj, errors.Join(err,
		c.reconcileLabels(obj, &localConditions),
		c.reconcileBindings(obj, &localConditions),
		c.updateStatus(obj, localConditions))
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	condition := metav1.Condition{Type: clusterRoleTemplateBindingDelete}

	if err := c.mgr.reconcileClusterMembershipBindingForDelete("", pkgrbac.GetRTBLabel(obj.ObjectMeta)); err != nil {
		c.s.AddCondition(&obj.Status.LocalConditions, condition, failedToDeleteClusterMembershipBinding, err)
		return nil, errors.Join(err, c.updateStatus(obj, obj.Status.LocalConditions))
	}
	if err := c.removeMGMTClusterScopedPrivilegesInProjectNamespace(obj); err != nil {
		c.s.AddCondition(&obj.Status.LocalConditions, condition, failedToDeleteMGMTClusterScopedPrivilegesInProjectNamespace, err)
		return nil, errors.Join(err, c.updateStatus(obj, obj.Status.LocalConditions))
	}

	if err := c.mgr.removeAuthV2Permissions(authprovisioningv2.CRTBRoleBindingID, obj); err != nil {
		c.s.AddCondition(&obj.Status.LocalConditions, condition, failedToDeleteAuthV2Permissions, err)
		return nil, errors.Join(err, c.updateStatus(obj, obj.Status.LocalConditions))
	}

	return nil, nil
}

func (c *crtbLifecycle) reconcileSubject(binding *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) (*v3.ClusterRoleTemplateBinding, error) {
	condition := metav1.Condition{Type: subjectExists}
	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
		return binding, nil
	}

	if binding.UserPrincipalName != "" && binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := c.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			c.s.AddCondition(localConditions, condition, failedToCreateUser, err)
			return binding, err
		}

		binding.UserName = user.Name
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
		return binding, nil
	}

	if binding.UserPrincipalName == "" && binding.UserName != "" {
		u, err := c.userLister.Get("", binding.UserName)
		if err != nil {
			c.s.AddCondition(localConditions, condition, failedToGetUser, err)
			return binding, err
		}
		for _, p := range u.PrincipalIDs {
			if strings.HasSuffix(p, binding.UserName) {
				binding.UserPrincipalName = p
				break
			}
		}
		c.s.AddCondition(localConditions, condition, subjectExists, nil)
		return binding, nil
	}

	c.s.AddCondition(localConditions, condition, crtbHasNoSubject, fmt.Errorf("CRTB has no subject"))

	return nil, fmt.Errorf("ClusterRoleTemplateBinding %v has no subject", binding.Name)
}

// When a CRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC
// Specifically:
// - ensure the subject can see the cluster in the mgmt API
// - if the subject was granted owner permissions for the clsuter, ensure they can create/update/delete the cluster
// - if the subject was granted privileges to mgmt plane resources that are scoped to the cluster, enforce those rules in the cluster's mgmt plane namespace
func (c *crtbLifecycle) reconcileBindings(binding *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) error {
	condition := metav1.Condition{Type: bindingExists}
	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		c.s.AddCondition(localConditions, condition, bindingExists, nil)
		return nil
	}

	clusterName := binding.ClusterName
	cluster, err := c.clusterLister.Get("", clusterName)
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToGetCluster, err)
		return err
	}
	if cluster == nil {
		err = fmt.Errorf("cannot create binding because cluster %s was not found", clusterName)
		c.s.AddCondition(localConditions, condition, clusterNotFound, err)
		return err
	}
	// if roletemplate is not builtin, check if it's inherited/cloned
	isOwnerRole, err := c.mgr.checkReferencedRoles(binding.RoleTemplateName, clusterContext, 0)
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToCheckReferencedRole, err)

		if apierrors.IsNotFound(err) {
			logrus.Warnf("ClusterRoleTemplateBinding %s sets a non-existing role template %s. Skipping.", binding.Name, binding.RoleTemplateName)
			return nil
		}

		return err
	}
	var clusterRoleName string
	if isOwnerRole {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%s-clusterowner", clusterName))
	} else {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%s-clustermember", clusterName))
	}

	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToBuildSubject, err)
		return err
	}
	if err := c.mgr.ensureClusterMembershipBinding(clusterRoleName, pkgrbac.GetRTBLabel(binding.ObjectMeta), cluster, isOwnerRole, subject); err != nil {
		c.s.AddCondition(localConditions, condition, failedToEnsureClusterMembershipBinding, err)
		return err
	}

	err = c.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, clusterManagementPlaneResources, subject, binding)
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToGrantManagementPlanePrivileges, err)
		return err
	}

	projects, err := c.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToListProjects, err)
		return err
	}
	for _, p := range projects {
		if p.DeletionTimestamp != nil {
			logrus.Warnf("Project %v is being deleted, not creating membership bindings", p.Name)
			continue
		}
		if err := c.mgr.grantManagementClusterScopedPrivilegesInProjectNamespace(binding.RoleTemplateName, p.Name, projectManagementPlaneResources, subject, binding); err != nil {
			c.s.AddCondition(localConditions, condition, failedToGrantManagementClusterScopedPrivilegesInProjectNamespace, err)
			return err
		}
	}
	c.s.AddCondition(localConditions, condition, bindingExists, nil)

	return nil
}

func (c *crtbLifecycle) removeMGMTClusterScopedPrivilegesInProjectNamespace(binding *v3.ClusterRoleTemplateBinding) error {
	projects, err := c.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		return err
	}
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, p := range projects {
		set := labels.Set(map[string]string{bindingKey: CrtbInProjectBindingOwner})
		rbs, err := c.rbLister.List(p.Name, set.AsSelector())
		if err != nil {
			return err
		}
		for _, rb := range rbs {
			logrus.Infof("[%v] Deleting rolebinding %v in namespace %v for crtb %v", ctrbMGMTController, rb.Name, p.Name, binding.Name)
			if err := c.rbClient.DeleteNamespaced(p.Name, rb.Name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *crtbLifecycle) reconcileLabels(binding *v3.ClusterRoleTemplateBinding, localConditions *[]metav1.Condition) error {
	/* Prior to 2.5, for every CRTB, following CRBs and RBs are created in the management clusters
		1. CRTB.UID is the label key for a CRB, CRTB.UID=memberhsip-binding-owner
	    2. CRTB.UID is label key for the RB, CRTB.UID=crtb-in-project-binding-owner (in the namespace of each project in the cluster that the user has access to)
	Using above labels, list the CRB and RB and update them to add a label with ns+name of CRTB
	*/
	condition := metav1.Condition{Type: labelsReconciled}

	if binding.Labels[RtbCrbRbLabelsUpdated] == "true" {
		c.s.AddCondition(localConditions, condition, labelsReconciled, nil)
		return nil
	}

	var returnErr error
	requirements, err := getLabelRequirements(binding.ObjectMeta)
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToGetRequirements, err)
		return err
	}

	set := labels.Set(map[string]string{string(binding.UID): MembershipBindingOwnerLegacy})
	crbs, err := c.crbLister.List(metav1.NamespaceAll, set.AsSelector().Add(requirements...))
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToGetClusterRoleBindings, err)
		return err
	}
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, crb := range crbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := c.crbClient.Get(crb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if crbToUpdate.Labels == nil {
				crbToUpdate.Labels = make(map[string]string)
			}
			crbToUpdate.Labels[bindingKey] = MembershipBindingOwner
			crbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := c.crbClient.Update(crbToUpdate)
			return err
		})
		if retryErr != nil {
			c.s.AddCondition(localConditions, condition, failedToUpdateClusterRoleBindings, retryErr)
		}
		returnErr = errors.Join(returnErr, retryErr)
	}

	set = map[string]string{string(binding.UID): CrtbInProjectBindingOwner}
	rbs, err := c.rbLister.List(metav1.NamespaceAll, set.AsSelector().Add(requirements...))
	if err != nil {
		c.s.AddCondition(localConditions, condition, failedToListRB, err)
		return err
	}

	for _, rb := range rbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			rbToUpdate, updateErr := c.rbClient.GetNamespaced(rb.Namespace, rb.Name, metav1.GetOptions{})
			if updateErr != nil {
				return updateErr
			}
			if rbToUpdate.Labels == nil {
				rbToUpdate.Labels = make(map[string]string)
			}
			rbToUpdate.Labels[bindingKey] = CrtbInProjectBindingOwner
			rbToUpdate.Labels[rtbLabelUpdated] = "true"
			_, err := c.rbClient.Update(rbToUpdate)
			return err
		})
		if retryErr != nil {
			c.s.AddCondition(localConditions, condition, failedToUpdateClusterRoleBindings, retryErr)
		}
		returnErr = errors.Join(returnErr, retryErr)
	}
	if returnErr != nil {
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
		crtbToUpdate.Labels[RtbCrbRbLabelsUpdated] = "true"
		_, err := c.crtbClient.Update(crtbToUpdate)
		return err
	})
	if retryErr != nil {
		c.s.AddCondition(localConditions, condition, failedToUpdateClusterRoleTemplateBindings, retryErr)
	}
	return retryErr
}

var timeNow = func() time.Time {
	return time.Now()
}

func (c *crtbLifecycle) updateStatus(crtb *v3.ClusterRoleTemplateBinding, localConditions []metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbFromCluster, err := c.crtbCache.Get(crtb.Namespace, crtb.Name)
		if err != nil {
			return err
		}
		if status.CompareConditions(crtbFromCluster.Status.LocalConditions, localConditions) {
			return nil
		}

		crtbFromCluster.Status.SummaryLocal = status.SummaryCompleted
		if crtbFromCluster.Status.SummaryRemote == status.SummaryCompleted {
			crtbFromCluster.Status.Summary = status.SummaryCompleted
		}
		for _, c := range localConditions {
			if c.Status != metav1.ConditionTrue {
				crtbFromCluster.Status.Summary = status.SummaryError
				crtbFromCluster.Status.SummaryLocal = status.SummaryError
				break
			}
		}

		crtbFromCluster.Status.LastUpdateTime = timeNow().Format(time.RFC3339)
		crtbFromCluster.Status.ObservedGenerationLocal = crtb.ObjectMeta.Generation
		crtbFromCluster.Status.LocalConditions = localConditions
		crtbFromCluster, err = c.crtbClient.UpdateStatus(crtbFromCluster)
		if err != nil {
			return err
		}

		return nil
	})
}
