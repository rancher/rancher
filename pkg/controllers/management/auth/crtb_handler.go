package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/controllers/management/authprovisioningv2"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/user"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
	// BadRoleReferences indicates issues with the roles referenced by the CRTB.
	BadRoleReferences = "BadRoleReferences"
	// BindingsExist is a success indicator. The CRTB-related bindings are all present and correct.
	BindingsExist = "BindingsExist"
	// CRTBExists
	CRTBExists = "CRTBExists"
	// FailedClusterMembershipBindingForDelete indicates that CRTB termination failed due to failure to delete the associated cluster membership binding
	FailedClusterMembershipBindingForDelete = "FailedClusterMembershipBindingForDelete"
	// FailedRemovalOfAuthV2Permissions indicates that CRTB termination failed due to failure of removing Auth V2 permissions
	FailedRemovalOfAuthV2Permissions = "FailedRemovalOfAuthV2Permissions"
	// FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace indicates that CRTB termination failed due to failure of removing cluster scoped privileges in the project namespace
	FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace = "FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace"
	// FailedToBuildSubject means that the controller did not have enough data in the CRTB to build a resource reference.
	FailedToBuildSubject = "FailedToBuildSubject"
	// FailedToEnsureClusterMembership means that the controller was unable to create the role binding providing the subject access to the cluster resource.
	FailedToEnsureClusterMembership = "FailedToEnsureClusterMembership"
	// FailedToGetCluster means that the cluster referenced by the CRTB is not found.
	FailedToGetCluster = "FailedToGetCluster"
	// FailedToGetClusterRoleBindings means that the controller was unable to retrieve the CRTB-related cluster role bindings to update.
	FailedToGetClusterRoleBindings = "FailedToGetClusterRoleBindings"
	// FailedToGetLabelRequirements indicates issues with the CRTB meta data preventing creation of label requirements.
	FailedToGetLabelRequirements = "FailedToGetLabelRequirements"
	// FailedToGetNamespace means that the controller was unable to find the project namespace referenced by the CRTB.
	FailedToGetNamespace = "FailedToGetNamespace"
	// FailedToGetRoleBindings means that the controller was unable to retrieve the CRTB-related role bindings to update.
	FailedToGetRoleBindings = "FailedToGetRoleBindings"
	// FailedToGetSubject means that the controller was unable to ensure the User referenced by the CRTB.
	FailedToGetSubject = "FailedToGetSubject"
	// FailedToGrantManagementClusterPrivileges means that the controller was unable to let the CRTB-related RBs grant proper permissions to project-scoped resources.
	FailedToGrantManagementClusterPrivileges = "FailedToGrantManagementClusterPrivileges"
	// FailedToGrantManagementPlanePrivileges means that the controller was unable to authorize the CRTB in the cluster it belongs to.
	FailedToGrantManagementPlanePrivileges = "FailedToGrantManagementPlanePrivileges"
	// FailedToUpdateCRTBLabels means the controller failed to update the CRTB labels indicating success of CRB/RB label updates.
	FailedToUpdateCRTBLabels = "FailedToUpdateCRTBLabels"
	// FailedToUpdateClusterRoleBindings means that the controller was unable to properly update the CRTB-related cluster role bindings.
	FailedToUpdateClusterRoleBindings = "FailedToUpdateClusterRoleBindings"
	// FailedToUpdateRoleBindings means that the controller was unable to properly update the CRTB-related role bindings.
	FailedToUpdateRoleBindings = "FailedToUpdateRoleBindings"
	// LabelsSet is a success indicator. The CRTB-related labels are all set.
	LabelsSet = "LabelsSet"
	// NoBindingsRequired is a success indicator.
	NoBindingsRequired = "NoBindingsRequired"
	// SubjectExists is a success indicator. The CRTB-related subject exists.
	SubjectExists = "SubjectExists"
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
)

var clusterManagementPlaneResources = map[string]string{
	"clusterscans":                "management.cattle.io",
	"catalogtemplates":            "management.cattle.io",
	"catalogtemplateversions":     "management.cattle.io",
	"clusteralertrules":           "management.cattle.io",
	"clusteralertgroups":          "management.cattle.io",
	"clustercatalogs":             "management.cattle.io",
	"clusterloggings":             "management.cattle.io",
	"clustermonitorgraphs":        "management.cattle.io",
	"clusterregistrationtokens":   "management.cattle.io",
	"clusterroletemplatebindings": "management.cattle.io",
	"etcdbackups":                 "management.cattle.io",
	"nodes":                       "management.cattle.io",
	"nodepools":                   "management.cattle.io",
	"notifiers":                   "management.cattle.io",
	"projects":                    "management.cattle.io",
	"etcdsnapshots":               "rke.cattle.io",
}

// Local interface abstracting the mgmtconv3.ClusterRoleTemplateBindingController down to
// necessities. The testsuite then provides a local mock implementation for itself.
type clusterRoleTemplateBindingController interface {
	UpdateStatus(*v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error)
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
	crtbClient    v3.ClusterRoleTemplateBindingInterface
	crtbClientM   clusterRoleTemplateBindingController
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	// If only the status has been updated and we have finished updating the status
	// (status.Summary != "InProgress") we don't need to perform a reconcile as nothing has
	// changed.
	if obj.Status.ObservedGeneration == obj.ObjectMeta.Generation &&
		obj.Status.Summary != SummaryInProgress {
		return obj, nil
	}
	returnError := errors.Join(
		c.setCRTBAsInProgress(obj),
		c.reconcileSubject(obj),
		c.reconcileBindings(obj),
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
		c.reconcileSubject(obj),
		c.reconcileLabels(obj),
		c.reconcileBindings(obj),
		c.setCRTBAsCompleted(obj),
	)
	return obj, returnError
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	returnError := errors.Join(
		c.setCRTBAsTerminating(obj),
		c.reconcileClusterMembershipBindingForDelete(obj),
		c.removeMGMTClusterScopedPrivilegesInProjectNamespace(obj),
		c.removeAuthV2Permissions(obj),
	)
	return obj, returnError
}

func (c *crtbLifecycle) reconcileSubject(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: SubjectExists}

	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
		addCondition(binding, condition, SubjectExists, binding.UserName, nil)
		return nil
	}

	if binding.UserPrincipalName != "" && binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := c.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			addCondition(binding, condition, FailedToGetSubject, binding.UserPrincipalName, err)
			return err
		}

		binding.UserName = user.Name
		addCondition(binding, condition, SubjectExists, binding.UserName, nil)
		return nil
	}

	if binding.UserPrincipalName == "" && binding.UserName != "" {
		u, err := c.userLister.Get("", binding.UserName)
		if err != nil {
			addCondition(binding, condition, FailedToGetSubject, binding.UserName, err)
			return err
		}
		for _, p := range u.PrincipalIDs {
			if strings.HasSuffix(p, binding.UserName) {
				binding.UserPrincipalName = p
				break
			}
		}
		addCondition(binding, condition, SubjectExists, binding.UserPrincipalName, nil)
		return nil
	}

	err := fmt.Errorf("ClusterRoleTemplateBinding %v has no subject", binding.Name)
	addCondition(binding, condition, FailedToGetSubject, binding.Name, err)
	return err
}

// When a CRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC
// Specifically:
// - ensure the subject can see the cluster in the mgmt API
// - if the subject was granted owner permissions for the clsuter, ensure they can create/update/delete the cluster
// - if the subject was granted privileges to mgmt plane resources that are scoped to the cluster, enforce those rules in the cluster's mgmt plane namespace
func (c *crtbLifecycle) reconcileBindings(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: BindingsExist}

	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		addCondition(binding, condition, NoBindingsRequired, binding.Name, nil)
		return nil
	}

	clusterName := binding.ClusterName
	cluster, err := c.clusterLister.Get("", clusterName)
	if err != nil {
		addCondition(binding, condition, FailedToGetCluster, binding.Name, err)
		return err
	}
	if cluster == nil {
		err := fmt.Errorf("cannot create binding because cluster %v was not found", clusterName)
		addCondition(binding, condition, FailedToGetCluster, binding.Name, err)
		return err
	}
	// if roletemplate is not builtin, check if it's inherited/cloned
	isOwnerRole, err := c.mgr.checkReferencedRoles(binding.RoleTemplateName, clusterContext, 0)
	if err != nil {
		addCondition(binding, condition, BadRoleReferences, binding.Name, err)
		return err
	}
	var clusterRoleName string
	if isOwnerRole {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clusterowner", clusterName))
	} else {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clustermember", clusterName))
	}

	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		addCondition(binding, condition, FailedToBuildSubject, binding.Name, err)
		return err
	}
	if err := c.mgr.ensureClusterMembershipBinding(clusterRoleName, pkgrbac.GetRTBLabel(binding.ObjectMeta), cluster, isOwnerRole, subject); err != nil {
		addCondition(binding, condition, FailedToEnsureClusterMembership, binding.Name, err)
		return err
	}

	err = c.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, clusterManagementPlaneResources, subject, binding)
	if err != nil {
		addCondition(binding, condition, FailedToGrantManagementPlanePrivileges, binding.Name, err)
		return err
	}

	projects, err := c.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		addCondition(binding, condition, FailedToGetNamespace, binding.Name, err)
		return err
	}
	for _, p := range projects {
		if p.DeletionTimestamp != nil {
			logrus.Warnf("Project %v is being deleted, not creating membership bindings", p.Name)
			continue
		}
		if err := c.mgr.grantManagementClusterScopedPrivilegesInProjectNamespace(binding.RoleTemplateName, p.Name, projectManagementPlaneResources, subject, binding); err != nil {
			addCondition(binding, condition, FailedToGrantManagementClusterPrivileges, binding.Name, err)
			return err
		}
	}

	addCondition(binding, condition, BindingsExist, binding.Name, nil)
	return nil
}

func (c *crtbLifecycle) reconcileClusterMembershipBindingForDelete(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: CRTBExists}

	err := c.mgr.reconcileClusterMembershipBindingForDelete("", pkgrbac.GetRTBLabel(binding.ObjectMeta))
	if err != nil {
		addCondition(binding, condition, FailedClusterMembershipBindingForDelete, binding.UserName, nil)
	}
	return err
}

func (c *crtbLifecycle) removeAuthV2Permissions(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: CRTBExists}

	err := c.mgr.removeAuthV2Permissions(authprovisioningv2.CRTBRoleBindingID, binding)
	if err != nil {
		addCondition(binding, condition, FailedRemovalOfAuthV2Permissions, binding.UserName, nil)
	}
	return err
}

func (c *crtbLifecycle) removeMGMTClusterScopedPrivilegesInProjectNamespace(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: CRTBExists}

	projects, err := c.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		addCondition(binding, condition, FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace, binding.UserName, nil)
		return err
	}
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, p := range projects {
		set := labels.Set(map[string]string{bindingKey: CrtbInProjectBindingOwner})
		rbs, err := c.rbLister.List(p.Name, set.AsSelector())
		if err != nil {
			addCondition(binding, condition, FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace, binding.UserName, nil)
			return err
		}
		for _, rb := range rbs {
			logrus.Infof("[%v] Deleting rolebinding %v in namespace %v for crtb %v", ctrbMGMTController, rb.Name, p.Name, binding.Name)
			if err := c.rbClient.DeleteNamespaced(p.Name, rb.Name, &v1.DeleteOptions{}); err != nil {
				addCondition(binding, condition, FailedRemovalOfMGMTClusterScopedPrivilegesInProjectNamespace, binding.UserName, nil)
				return err
			}
		}
	}
	return nil
}

func (c *crtbLifecycle) reconcileLabels(binding *v3.ClusterRoleTemplateBinding) error {
	condition := metav1.Condition{Type: LabelsSet}

	/* Prior to 2.5, for every CRTB, following CRBs and RBs are created in the management clusters
		1. CRTB.UID is the label key for a CRB, CRTB.UID=memberhsip-binding-owner
	    2. CRTB.UID is label key for the RB, CRTB.UID=crtb-in-project-binding-owner (in the namespace of each project in the cluster that the user has access to)
	Using above labels, list the CRB and RB and update them to add a label with ns+name of CRTB
	*/
	if binding.Labels[RtbCrbRbLabelsUpdated] == "true" {
		addCondition(binding, condition, LabelsSet, binding.Name, nil)
		return nil
	}

	var returnErr error
	requirements, err := getLabelRequirements(binding.ObjectMeta)
	if err != nil {
		addCondition(binding, condition, FailedToGetLabelRequirements, binding.Name, err)
		return err
	}

	set := labels.Set(map[string]string{string(binding.UID): MembershipBindingOwnerLegacy})
	crbs, err := c.crbLister.List(v1.NamespaceAll, set.AsSelector().Add(requirements...))
	if err != nil {
		addCondition(binding, condition, FailedToGetClusterRoleBindings, binding.Name, err)
		return err
	}
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, crb := range crbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			crbToUpdate, updateErr := c.crbClient.Get(crb.Name, v1.GetOptions{})
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
			addCondition(binding, condition, FailedToUpdateClusterRoleBindings, binding.Name, retryErr)
			returnErr = errors.Join(returnErr, retryErr)
		}
	}

	set = map[string]string{string(binding.UID): CrtbInProjectBindingOwner}
	rbs, err := c.rbLister.List(v1.NamespaceAll, set.AsSelector().Add(requirements...))
	if err != nil {
		addCondition(binding, condition, FailedToGetRoleBindings, binding.Name, err)
		return err
	}

	for _, rb := range rbs {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			rbToUpdate, updateErr := c.rbClient.GetNamespaced(rb.Namespace, rb.Name, v1.GetOptions{})
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
			addCondition(binding, condition, FailedToUpdateRoleBindings, binding.Name, retryErr)
			returnErr = errors.Join(returnErr, retryErr)
		}
	}
	if returnErr != nil {
		// No condition here, already collected in the retries
		return returnErr
	}

	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crtbToUpdate, updateErr := c.crtbClient.GetNamespaced(binding.Namespace, binding.Name, v1.GetOptions{})
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
