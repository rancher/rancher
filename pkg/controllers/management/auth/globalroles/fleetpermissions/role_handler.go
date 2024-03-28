package fleetpermissions

import (
	"fmt"

	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
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

type RoleHandler struct {
	crClient rbacv1.ClusterRoleController
	crCache  rbacv1.ClusterRoleCache
	fwCache  mgmtcontroller.FleetWorkspaceCache
}

func NewRoleHandler(management *config.ManagementContext) *RoleHandler {
	return &RoleHandler{
		crClient: management.Wrangler.RBAC.ClusterRole(),
		crCache:  management.Wrangler.RBAC.ClusterRole().Cache(),
		fwCache:  management.Wrangler.Mgmt.FleetWorkspace().Cache(),
	}
}

// ReconcileFleetWorkspacePermissions reconciles backing ClusterRoles created for granting permission to fleet workspaces.
func (h *RoleHandler) ReconcileFleetWorkspacePermissions(globalRole *v3.GlobalRole) error {
	if globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs == nil &&
		globalRole.InheritedFleetWorkspacePermissions.ResourceRules == nil {
		return nil
	}

	if err := h.reconcileRulesClusterRole(globalRole); err != nil {
		return fmt.Errorf("error reconciling fleet permissions cluster role: %w", err)
	}
	if err := h.reconcileVerbsClusterRole(globalRole); err != nil {
		return fmt.Errorf("error reconciling fleet workspace verbs cluster role: %w", err)
	}

	return nil
}

func (h *RoleHandler) reconcileRulesClusterRole(globalRole *v3.GlobalRole) error {
	crName := fleetWorkspaceClusterRulesPrefix + globalRole.Name

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

func (h *RoleHandler) reconcileVerbsClusterRole(globalRole *v3.GlobalRole) error {
	if globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs == nil || len(globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs) == 0 {
		return nil
	}
	crName := fleetWorkspaceVerbsPrefix + globalRole.Name
	fleetWorkspaces, err := h.fwCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("unable to list fleetWorkspaces when reconciling globalRole %s: %w", globalRole.Name, err)
	}

	var workspacesNames []string
	for _, fleetWorkspace := range fleetWorkspaces {
		if fleetWorkspace.Name != localFleetWorkspace {
			workspacesNames = append(workspacesNames, fleetWorkspace.Name)
		}
	}
	rules := []v1.PolicyRule{
		{
			Verbs:         globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs,
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{"fleetworkspaces"},
			ResourceNames: workspacesNames,
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
