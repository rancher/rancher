package fleetpermissions

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	rbacv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	GRBFleetWorkspaceOwnerLabel      = "authz.management.cattle.io/grb-fw-owner"
	localFleetWorkspace              = "fleet-local"
	fleetWorkspaceClusterRulesPrefix = "fwcr-"
	fleetWorkspaceVerbsPrefix        = "fwv-"
)

type Handler struct {
	crClient  rbacv1.ClusterRoleController
	crCache   rbacv1.ClusterRoleCache
	crbClient rbacv1.ClusterRoleBindingController
	crbCache  rbacv1.ClusterRoleBindingCache
	grCache   mgmtcontroller.GlobalRoleCache
	rbClient  rbacv1.RoleBindingController
	rbCache   rbacv1.RoleBindingCache
	fwCache   mgmtcontroller.FleetWorkspaceCache
}

func NewHandler(management *config.ManagementContext) *Handler {
	return &Handler{
		crClient:  management.Wrangler.RBAC.ClusterRole(),
		crCache:   management.Wrangler.RBAC.ClusterRole().Cache(),
		crbClient: management.Wrangler.RBAC.ClusterRoleBinding(),
		crbCache:  management.Wrangler.RBAC.ClusterRoleBinding().Cache(),
		grCache:   management.Wrangler.Mgmt.GlobalRole().Cache(),
		rbClient:  management.Wrangler.RBAC.RoleBinding(),
		rbCache:   management.Wrangler.RBAC.RoleBinding().Cache(),
		fwCache:   management.Wrangler.Mgmt.FleetWorkspace().Cache(),
	}
}

// ReconcileFleetWorkspacePermissions creates or updates backing ClusterRoles and bindings for fleet workspace permissions.
// Permissions specified in InheritedFleetWorkspacePermissions will apply to all fleet workspaces except local.
// InheritedFleetWorkspacePermissions.ResourceRules applies to the namespace of the workspace.
// InheritedFleetWorkspacePermissions.WorkspaceVerbs applies to the cluster-wide fleetworkspace resources.
func (h *Handler) ReconcileFleetWorkspacePermissions(globalRoleBinding *v3.GlobalRoleBinding) error {
	globalRole, err := h.grCache.Get(globalRoleBinding.GlobalRoleName)
	if err != nil {
		return fmt.Errorf("unable to get globalRole: %w", err)
	}
	if globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs == nil &&
		globalRole.InheritedFleetWorkspacePermissions.ResourceRules == nil {
		return nil
	}
	fleetWorkspaces, err := h.fwCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("unable to list fleetWorkspaces when reconciling globalRoleBinding %s: %w", globalRoleBinding.Name, err)
	}

	err = h.reconcilePermissionsResourceRules(globalRoleBinding, globalRole, fleetWorkspaces)
	if err != nil {
		return fmt.Errorf("error reconciling fleet permissions rules: %w", err)
	}

	err = h.reconcilePermissionsWorkspaceVerbs(globalRoleBinding, globalRole, fleetWorkspaces)
	if err != nil {
		return fmt.Errorf("error reconciling fleet workspace verbs: %w", err)
	}

	return nil
}

func (h *Handler) reconcilePermissionsResourceRules(globalRoleBinding *v3.GlobalRoleBinding, globalRole *v3.GlobalRole, fleetWorkspaces []*v3.FleetWorkspace) error {
	fleetWorkspacePermissionName := fleetWorkspaceClusterRulesPrefix + globalRole.Name

	err := h.reconcileRulesClusterRole(globalRole, globalRoleBinding.Name, fleetWorkspacePermissionName)
	if err != nil {
		return err
	}

	var returnError error
	subject := rbac.GetGRBSubject(globalRoleBinding)
	roleref := v1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     fleetWorkspacePermissionName,
	}
	subjects := []v1.Subject{
		subject,
	}

	for _, fleetWorkspace := range fleetWorkspaces {
		rbName := subject.Name + "-" + fleetWorkspacePermissionName + "-" + fleetWorkspace.Name
		if fleetWorkspace.Name == localFleetWorkspace {
			continue
		}

		rb, err := h.rbCache.Get(fleetWorkspace.Name, rbName)
		if err != nil && !apierrors.IsNotFound(err) {
			returnError = multierror.Append(returnError, err)
			continue
		}
		if apierrors.IsNotFound(err) {
			_, err = h.rbClient.Create(
				&v1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rbName,
						Namespace: fleetWorkspace.Name,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: v3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
								Kind:       v3.GlobalRoleBindingGroupVersionKind.Kind,
								Name:       globalRoleBinding.Name,
								UID:        globalRoleBinding.UID,
							},
						},
						Labels: map[string]string{
							GRBFleetWorkspaceOwnerLabel: globalRoleBinding.Name,
							controllers.K8sManagedByKey: controllers.ManagerValue,
						},
					},
					RoleRef:  roleref,
					Subjects: subjects,
				})
			if err != nil && !apierrors.IsNotFound(err) {
				returnError = multierror.Append(returnError, err)
			}
		} else if !reflect.DeepEqual(rb.Subjects, subjects) {
			rb.Subjects = subjects
			_, err := h.rbClient.Update(rb)
			if err != nil {
				returnError = multierror.Append(returnError, err)
			}
		}
	}

	return returnError
}

func (h *Handler) reconcileRulesClusterRole(globalRole *v3.GlobalRole, grbName string, crName string) error {
	cr, err := h.crCache.Get(crName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		_, err := h.crClient.Create(&v1.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: crName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: v3.GlobalRoleGroupVersionKind.GroupVersion().String(),
						Kind:       v3.GlobalRoleGroupVersionKind.Kind,
						Name:       globalRole.Name,
						UID:        globalRole.UID,
					},
				},
				Labels: map[string]string{
					GRBFleetWorkspaceOwnerLabel: grbName,
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			Rules: globalRole.InheritedFleetWorkspacePermissions.ResourceRules,
		})

		return err
	}

	if !equality.Semantic.DeepEqual(cr.Rules, globalRole.InheritedFleetWorkspacePermissions.ResourceRules) {
		cr.Rules = globalRole.InheritedFleetWorkspacePermissions.ResourceRules
		_, err := h.crClient.Update(cr)

		return err
	}

	return err
}

func (h *Handler) reconcilePermissionsWorkspaceVerbs(globalRoleBinding *v3.GlobalRoleBinding, globalRole *apimgmtv3.GlobalRole, fleetWorkspaces []*apimgmtv3.FleetWorkspace) error {
	var workspacesNames []string
	if globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs == nil || len(globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs) == 0 {
		return nil
	}
	for _, fleetWorkspace := range fleetWorkspaces {
		if fleetWorkspace.Name != localFleetWorkspace {
			workspacesNames = append(workspacesNames, fleetWorkspace.Name)
		}
	}
	crName := fleetWorkspaceVerbsPrefix + globalRole.Name
	err := h.reconcileVerbsClusterRole(globalRole, globalRoleBinding.Name, crName, workspacesNames)
	if err != nil {
		return err
	}

	subject := rbac.GetGRBSubject(globalRoleBinding)
	subjects := []v1.Subject{subject}
	crbName := crName + "-" + subject.Name
	crb, err := h.crbCache.Get(crbName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		_, err = h.crbClient.Create(
			&v1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: crName + "-" + subject.Name,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: v3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
							Kind:       v3.GlobalRoleBindingGroupVersionKind.Kind,
							Name:       globalRoleBinding.Name,
							UID:        globalRoleBinding.UID,
						},
					},
					Labels: map[string]string{
						GRBFleetWorkspaceOwnerLabel: globalRoleBinding.Name,
						controllers.K8sManagedByKey: controllers.ManagerValue,
					},
				},
				RoleRef: v1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     crName,
				},
				Subjects: subjects,
			})

		return err
	}
	if !reflect.DeepEqual(crb.Subjects, subjects) {
		crb.Subjects = subjects
		_, err := h.crbClient.Update(crb)

		return err
	}

	return nil
}

func (h *Handler) reconcileVerbsClusterRole(globalRole *v3.GlobalRole, grbName string, crName string, resourceNames []string) error {
	rules := []v1.PolicyRule{
		{
			Verbs:         globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs,
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{"fleetworkspaces"},
			ResourceNames: resourceNames,
		},
	}
	cr, err := h.crCache.Get(crName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if apierrors.IsNotFound(err) {
		_, err := h.crClient.Create(&v1.ClusterRole{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: crName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: v3.GlobalRoleGroupVersionKind.GroupVersion().String(),
						Kind:       v3.GlobalRoleGroupVersionKind.Kind,
						Name:       globalRole.Name,
						UID:        globalRole.UID,
					},
				},
				Labels: map[string]string{
					GRBFleetWorkspaceOwnerLabel: grbName,
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			Rules: rules,
		})

		return err
	} else if !equality.Semantic.DeepEqual(cr.Rules, rules) {
		cr.Rules = rules
		_, err := h.crClient.Update(cr)

		return err
	}

	return nil

}
