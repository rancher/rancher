package globalroles

import (
	"fmt"
	"reflect"

	wrangler "github.com/rancher/wrangler/v2/pkg/name"

	"github.com/hashicorp/go-multierror"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	rbacv1 "github.com/rancher/wrangler/v2/pkg/generated/controllers/rbac/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	errReconcileResourceRulesBinding  = fmt.Errorf("reconciling fleet role bindings failed")
	errReconcileWorkspaceVerbsBinding = fmt.Errorf("reconciling fleet workspace verbs cluster role binding failed")
)

// fleetWorkspaceBindingHandler manages Bindings created for the InheritedFleetWorkspacePermissions field.
type fleetWorkspaceBindingHandler struct {
	crbClient rbacv1.ClusterRoleBindingController
	crbCache  rbacv1.ClusterRoleBindingCache
	crCache   rbacv1.ClusterRoleCache
	grCache   mgmtcontroller.GlobalRoleCache
	rbClient  rbacv1.RoleBindingController
	rbCache   rbacv1.RoleBindingCache
	fwCache   mgmtcontroller.FleetWorkspaceCache
}

func newFleetWorkspaceBindingHandler(management *config.ManagementContext) *fleetWorkspaceBindingHandler {
	return &fleetWorkspaceBindingHandler{
		crbClient: management.Wrangler.RBAC.ClusterRoleBinding(),
		crbCache:  management.Wrangler.RBAC.ClusterRoleBinding().Cache(),
		grCache:   management.Wrangler.Mgmt.GlobalRole().Cache(),
		rbClient:  management.Wrangler.RBAC.RoleBinding(),
		rbCache:   management.Wrangler.RBAC.RoleBinding().Cache(),
		fwCache:   management.Wrangler.Mgmt.FleetWorkspace().Cache(),
		crCache:   management.Wrangler.RBAC.ClusterRole().Cache(),
	}
}

// ReconcileFleetWorkspacePermissionsBindings reconciles backing RoleBindings and ClusterRoleBindings created for granting permission
// to fleet workspaces.
func (h *fleetWorkspaceBindingHandler) reconcileFleetWorkspacePermissionsBindings(globalRoleBinding *v3.GlobalRoleBinding) error {
	globalRole, err := h.grCache.Get(globalRoleBinding.GlobalRoleName)
	if err != nil {
		return fmt.Errorf("couldn't get globalRole: %w", err)
	}
	var returnErr error
	if err = h.reconcileResourceRulesBindings(globalRoleBinding, globalRole); err != nil {
		returnErr = multierror.Append(returnErr, errReconcileResourceRulesBinding, err)
	}
	if err = h.reconcileWorkspaceVerbsBindings(globalRoleBinding, globalRole); err != nil {
		returnErr = multierror.Append(returnErr, errReconcileWorkspaceVerbsBinding, err)
	}

	return returnErr
}

func (h *fleetWorkspaceBindingHandler) reconcileResourceRulesBindings(grb *v3.GlobalRoleBinding, gr *v3.GlobalRole) error {
	fleetWorkspaces, err := h.fwCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("couldn't list fleetWorkspaces: %w", err)
	}

	var returnError error
	for _, fleetWorkspace := range fleetWorkspaces {
		if fleetWorkspace.Name == localFleetWorkspace {
			continue
		}
		desiredRB := backingRoleBinding(grb, gr, fleetWorkspace.Name)
		rb, err := h.rbCache.Get(fleetWorkspace.Name, wrangler.SafeConcatName(grb.Name))
		if err != nil {
			if !apierrors.IsNotFound(err) {
				returnError = multierror.Append(returnError, fmt.Errorf("couldn't get RoleBinding %s : %w", wrangler.SafeConcatName(grb.Name), err))
				continue
			}
			if gr.InheritedFleetWorkspacePermissions != nil && gr.InheritedFleetWorkspacePermissions.ResourceRules != nil {
				_, err = h.crCache.Get(desiredRB.RoleRef.Name)
				if err != nil {
					returnError = multierror.Append(returnError, fmt.Errorf("couldn't get ClusterRole: %w", err))
					continue
				}
				_, err = h.rbClient.Create(desiredRB)
				if err != nil {
					returnError = multierror.Append(returnError, fmt.Errorf("couldn't create RoleBinding: %w", err))
				}
			}
			continue
		}

		if gr.InheritedFleetWorkspacePermissions == nil || gr.InheritedFleetWorkspacePermissions.ResourceRules == nil {
			err := h.rbClient.Delete(rb.Namespace, rb.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				returnError = multierror.Append(returnError, fmt.Errorf("couldn't clean up RoleBinding: %w", err))
			}
			continue
		}
		if !reflect.DeepEqual(rb.Subjects, desiredRB.Subjects) ||
			!reflect.DeepEqual(rb.RoleRef, desiredRB.RoleRef) {
			// undo modifications if rb has changed.
			err := h.rbClient.Delete(rb.Namespace, rb.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				returnError = multierror.Append(returnError, fmt.Errorf("couldn't delete RoleBinding: %w", err))
				continue
			}
			_, err = h.rbClient.Create(desiredRB)
			if err != nil {
				returnError = multierror.Append(returnError, fmt.Errorf("couldn't create RoleBinding: %w", err))
			}
		}
	}

	return returnError
}

func (h *fleetWorkspaceBindingHandler) reconcileWorkspaceVerbsBindings(grb *v3.GlobalRoleBinding, gr *apimgmtv3.GlobalRole) error {
	crbName := wrangler.SafeConcatName(grb.Name, fleetWorkspaceVerbsName)
	desiredCRB := backingClusterRoleBinding(grb, gr, crbName)

	crb, err := h.crbCache.Get(crbName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("couldn't get ClusterRoleBinding: %w", err)
		}
		if gr.InheritedFleetWorkspacePermissions != nil && gr.InheritedFleetWorkspacePermissions.ResourceRules != nil {
			_, err = h.crCache.Get(desiredCRB.RoleRef.Name)
			if err != nil {
				return fmt.Errorf("couldn't get ClusterRole: %w", err)
			}
			_, err = h.crbClient.Create(desiredCRB)
			if err != nil {
				return fmt.Errorf("couldn't create ClusterRoleBinding: %w", err)
			}
		}
		return nil
	}

	if gr.InheritedFleetWorkspacePermissions == nil || gr.InheritedFleetWorkspacePermissions.ResourceRules == nil {
		err := h.crbClient.Delete(crbName, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("couldn't clean up ClusterRoleBinding: %w", err)
		}
		return nil
	}
	if !reflect.DeepEqual(crb.Subjects, desiredCRB.Subjects) ||
		!reflect.DeepEqual(crb.RoleRef, desiredCRB.RoleRef) {
		// undo modifications if crb has changed.
		err := h.crbClient.Delete(crb.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("couldn't delete ClusterRoleBinding: %w", err)
		}
		_, err = h.crbClient.Create(desiredCRB)

		return err
	}

	return nil
}

func backingClusterRoleBinding(grb *v3.GlobalRoleBinding, gr *v3.GlobalRole, crbName string) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: crbName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
					Kind:       v3.GlobalRoleBindingGroupVersionKind.Kind,
					Name:       grb.Name,
					UID:        grb.UID,
				},
			},
			Labels: map[string]string{
				grbOwnerLabel:               wrangler.SafeConcatName(grb.Name),
				controllers.K8sManagedByKey: controllers.ManagerValue,
			},
		},
		RoleRef: v1.RoleRef{
			APIGroup: v1.GroupName,
			Kind:     "ClusterRole",
			Name:     wrangler.SafeConcatName(gr.Name, fleetWorkspaceVerbsName),
		},
		Subjects: []v1.Subject{rbac.GetGRBSubject(grb)},
	}
}

func backingRoleBinding(grb *v3.GlobalRoleBinding, gb *v3.GlobalRole, fwName string) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wrangler.SafeConcatName(grb.Name),
			Namespace: fwName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v3.GlobalRoleBindingGroupVersionKind.GroupVersion().String(),
					Kind:       v3.GlobalRoleBindingGroupVersionKind.Kind,
					Name:       grb.Name,
					UID:        grb.UID,
				},
			},
			Labels: map[string]string{
				grbOwnerLabel:                 wrangler.SafeConcatName(grb.Name),
				fleetWorkspacePermissionLabel: "true",
				controllers.K8sManagedByKey:   controllers.ManagerValue,
			},
		},
		RoleRef: v1.RoleRef{
			APIGroup: v1.GroupName,
			Kind:     "ClusterRole",
			Name:     wrangler.SafeConcatName(gb.Name, fleetWorkspaceClusterRulesName),
		},
		Subjects: []v1.Subject{
			rbac.GetGRBSubject(grb),
		},
	}
}
