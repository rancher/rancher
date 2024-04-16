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
func (h *fleetWorkspaceRoleHandler) reconcileFleetWorkspacePermissions(gr *v3.GlobalRole) error {
	if err := h.reconcileResourceRules(gr); err != nil {
		return fmt.Errorf("error reconciling fleet permissions cluster role: %w", err)
	}
	if err := h.reconcileWorkspaceVerbs(gr); err != nil {
		return fmt.Errorf("error reconciling fleet workspace verbs cluster role: %w", err)
	}

	return nil
}

func (h *fleetWorkspaceRoleHandler) reconcileResourceRules(gr *v3.GlobalRole) error {
	crName := wrangler.SafeConcatName(gr.Name, fleetWorkspaceClusterRulesName)
	cr, err := h.crCache.Get(crName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		if gr.InheritedFleetWorkspacePermissions.ResourceRules != nil {
			_, err := h.crClient.Create(backingResourceRulesClusterRole(gr, crName))
			return err
		}
		return nil
	}

	if gr.InheritedFleetWorkspacePermissions.ResourceRules == nil {
		err := h.crClient.Delete(crName, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	if !equality.Semantic.DeepEqual(cr.Rules, gr.InheritedFleetWorkspacePermissions.ResourceRules) {
		// undo modifications if cr has changed
		cr.Rules = gr.InheritedFleetWorkspacePermissions.ResourceRules
		_, err := h.crClient.Update(cr)

		return err
	}

	return err
}

func (h *fleetWorkspaceRoleHandler) reconcileWorkspaceVerbs(gr *v3.GlobalRole) error {
	crName := wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName)
	cr, err := h.crCache.Get(crName)
	crMissing := apierrors.IsNotFound(err)
	if err != nil && !crMissing {
		return err
	}
	if gr.InheritedFleetWorkspacePermissions.WorkspaceVerbs == nil {
		if !crMissing {
			err := h.crClient.Delete(crName, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	workspacesNames, err := h.fleetWorkspaceNames()
	if err != nil {
		return err
	}
	if len(workspacesNames) == 0 {
		// skip if there are no workspaces besides local
		return nil
	}
	desiredCR := backingWorkspaceVerbsClusterRole(gr, crName, workspacesNames)
	if crMissing {
		_, err := h.crClient.Create(desiredCR)

		return err
	} else if !equality.Semantic.DeepEqual(cr.Rules, desiredCR.Rules) {
		// undo modifications if cr has changed
		cr.Rules = desiredCR.Rules
		_, err := h.crClient.Update(cr)

		return err
	}

	return nil
}

func backingResourceRulesClusterRole(gr *v3.GlobalRole, crName string) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v3.GlobalRoleGroupVersionKind.GroupVersion().String(),
					Kind:       v3.GlobalRoleGroupVersionKind.Kind,
					Name:       gr.Name,
					UID:        gr.UID,
				},
			},
			Labels: map[string]string{
				grOwnerLabel:                wrangler.SafeConcatName(gr.Name),
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
		},
		Rules: gr.InheritedFleetWorkspacePermissions.ResourceRules,
	}
}

func backingWorkspaceVerbsClusterRole(gr *v3.GlobalRole, crName string, workspaceNames []string) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: crName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v3.GlobalRoleGroupVersionKind.GroupVersion().String(),
					Kind:       v3.GlobalRoleGroupVersionKind.Kind,
					Name:       gr.Name,
					UID:        gr.UID,
				},
			},
			Labels: map[string]string{
				grOwnerLabel:                wrangler.SafeConcatName(gr.Name),
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
		},
		Rules: []v1.PolicyRule{
			{
				Verbs:         gr.InheritedFleetWorkspacePermissions.WorkspaceVerbs,
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{"fleetworkspaces"},
				ResourceNames: workspaceNames,
			},
		},
	}
}

func (h *fleetWorkspaceRoleHandler) fleetWorkspaceNames() ([]string, error) {
	fleetWorkspaces, err := h.fwCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list fleetWorkspaces when reconciling globalRole: %w", err)
	}

	var workspacesNames []string
	for _, fleetWorkspace := range fleetWorkspaces {
		if fleetWorkspace.Name != localFleetWorkspace {
			workspacesNames = append(workspacesNames, fleetWorkspace.Name)
		}
	}

	return workspacesNames, nil
}
