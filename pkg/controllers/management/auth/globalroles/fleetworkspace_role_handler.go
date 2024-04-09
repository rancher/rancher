package globalroles

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers"

	wrangler "github.com/rancher/wrangler/v2/pkg/name"

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
	localFleetWorkspace            = "fleet-local"
	fleetWorkspaceClusterRulesName = "fwcr"
	fleetWorkspaceVerbsName        = "fwv"
)

// fleetWorkspaceRoleHandler manages ClusterRoles created for the InheritedFleetWorkspacePermissions field. It manages 2 roles:
// - 1) CR for ResourceRules
// - 2) CR for WorkspaceVerbs
type fleetWorkspaceRoleHandler struct {
	crClient rbacv1.ClusterRoleController
	crCache  rbacv1.ClusterRoleCache
	fwCache  mgmtcontroller.FleetWorkspaceCache
}

func newFleetWorkspaceRoleHandler(management *config.ManagementContext) *fleetWorkspaceRoleHandler {
	return &fleetWorkspaceRoleHandler{
		crClient: management.Wrangler.RBAC.ClusterRole(),
		crCache:  management.Wrangler.RBAC.ClusterRole().Cache(),
		fwCache:  management.Wrangler.Mgmt.FleetWorkspace().Cache(),
	}
}

// ReconcileFleetWorkspacePermissions reconciles backing ClusterRoles created for granting permission to fleet workspaces.
func (h *fleetWorkspaceRoleHandler) reconcileFleetWorkspacePermissions(globalRole *v3.GlobalRole) error {
	if globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs == nil &&
		globalRole.InheritedFleetWorkspacePermissions.ResourceRules == nil {
		return nil
	}

	if err := h.reconcileResourceRules(globalRole); err != nil {
		return fmt.Errorf("error reconciling fleet permissions cluster role: %w", err)
	}
	if err := h.reconcileWorkspaceVerbs(globalRole); err != nil {
		return fmt.Errorf("error reconciling fleet workspace verbs cluster role: %w", err)
	}

	return nil
}

func (h *fleetWorkspaceRoleHandler) reconcileResourceRules(globalRole *v3.GlobalRole) error {
	crName := wrangler.SafeConcatName(globalRole.Name, fleetWorkspaceClusterRulesName)
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
					grOwnerLabel:                wrangler.SafeConcatName(globalRole.Name),
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			Rules: globalRole.InheritedFleetWorkspacePermissions.ResourceRules,
		})

		return err
	}

	if !equality.Semantic.DeepEqual(cr.Rules, globalRole.InheritedFleetWorkspacePermissions.ResourceRules) {
		// undo modifications if cr has changed
		cr.Rules = globalRole.InheritedFleetWorkspacePermissions.ResourceRules
		_, err := h.crClient.Update(cr)

		return err
	}

	return err
}

func (h *fleetWorkspaceRoleHandler) reconcileWorkspaceVerbs(globalRole *v3.GlobalRole) error {
	if globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs == nil || len(globalRole.InheritedFleetWorkspacePermissions.WorkspaceVerbs) == 0 {
		return nil
	}
	crName := wrangler.SafeConcatName(globalRole.Name, fleetWorkspaceVerbsName)
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
	if workspacesNames == nil {
		// skip if there are no workspaces besides local
		return nil
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
				Labels: map[string]string{
					grOwnerLabel:                wrangler.SafeConcatName(globalRole.Name),
					controllers.K8sManagedByKey: controllers.ManagerValue,
				},
			},
			Rules: rules,
		})

		return err
	} else if !equality.Semantic.DeepEqual(cr.Rules, rules) {
		// undo modifications if cr has changed
		cr.Rules = rules
		_, err := h.crClient.Update(cr)

		return err
	}

	return nil
}
