package rbac

import (
	"context"
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	corew "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const inheritedNamespacedRulesHandlerName = "inherited-namespaced-rules-sync"

// inheritedNamespacedRulesHandler creates the Roles and RoleBindings that GlobalRoles with
// InheritedNamespacedRules define for namespaces of this cluster. The GlobalRole and
// GlobalRoleBinding controllers on the leader replica create these resources for namespaces
// that already exist, but they cannot observe namespaces created later: downstream namespace
// events are only seen by the replica that owns the cluster, and a wrangler enqueue never
// crosses replicas. This handler runs on the owner replica and reconciles the resources for
// its own cluster directly. It intentionally mirrors the resource shapes produced by
// pkg/controllers/management/auth/globalroles so both writers converge on the same objects,
// and leaves purging of stale resources to the leader-side controllers.
type inheritedNamespacedRulesHandler struct {
	grCache      mgmtv3.GlobalRoleCache
	grbCache     mgmtv3.GlobalRoleBindingCache
	roles        wrbacv1.RoleClient
	roleCache    wrbacv1.RoleCache
	roleBindings wrbacv1.RoleBindingClient
	rbCache      wrbacv1.RoleBindingCache
	clusterName  string
}

// RegisterInheritedNamespacedRulesHandler wires the handler onto the cluster's namespace
// controller. Split from Register so tests can run the owner-plane registration without the
// rest of the user context.
func RegisterInheritedNamespacedRulesHandler(
	ctx context.Context,
	namespaces corew.NamespaceController,
	grCache mgmtv3.GlobalRoleCache,
	grbCache mgmtv3.GlobalRoleBindingCache,
	roles wrbacv1.RoleController,
	roleBindings wrbacv1.RoleBindingController,
	clusterName string,
) {
	// InheritedNamespacedRules apply to downstream clusters only; the leader-side GlobalRole and
	// GlobalRoleBinding controllers deliberately exclude the local cluster, and so must this handler.
	if clusterName == "local" {
		return
	}
	h := &inheritedNamespacedRulesHandler{
		grCache:      grCache,
		grbCache:     grbCache,
		roles:        roles,
		roleCache:    roles.Cache(),
		roleBindings: roleBindings,
		rbCache:      roleBindings.Cache(),
		clusterName:  clusterName,
	}
	namespaces.OnChange(ctx, inheritedNamespacedRulesHandlerName, h.onNamespaceChange)
}

func (h *inheritedNamespacedRulesHandler) onNamespaceChange(_ string, ns *corev1.Namespace) (*corev1.Namespace, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return ns, nil
	}

	grs, err := h.grCache.GetByIndex(pkgrbac.GRDownstreamNSIndex, ns.Name)
	if err != nil {
		return ns, fmt.Errorf("failed to list GlobalRoles for namespace %s: %w", ns.Name, err)
	}

	var returnError error
	for _, gr := range grs {
		if _, ok := gr.InheritedNamespacedRules[ns.Name]; !ok {
			continue
		}
		if err := h.ensureRole(gr, ns.Name); err != nil {
			returnError = errors.Join(returnError, err)
			continue
		}

		grbs, err := h.grbCache.GetByIndex(pkgrbac.GRBGlobalRoleIndex, gr.Name)
		if err != nil {
			returnError = errors.Join(returnError, fmt.Errorf("failed to list GlobalRoleBindings for GlobalRole %s: %w", gr.Name, err))
			continue
		}
		for _, grb := range grbs {
			if err := h.ensureRoleBinding(grb, ns.Name); err != nil {
				returnError = errors.Join(returnError, err)
			}
		}
	}

	return ns, returnError
}

// ensureRole creates or updates the Role for a GlobalRole's inherited rules in the namespace.
func (h *inheritedNamespacedRulesHandler) ensureRole(gr *v3.GlobalRole, ns string) error {
	desired := pkgrbac.BuildInheritedRole(gr, ns)

	// A matching cached Role means there is nothing to write; skip the API round-trip.
	if cached, err := h.roleCache.Get(ns, desired.Name); err == nil && pkgrbac.AreRolesSame(cached, desired) {
		return nil
	}

	_, err := pkgrbac.CreateOrUpdateNamespacedResource(desired, h.roles, pkgrbac.AreRolesSame)
	if err != nil {
		return fmt.Errorf("failed to ensure Role for GlobalRole %s in namespace %s in cluster %s: %w", gr.Name, ns, h.clusterName, err)
	}
	return nil
}

// ensureRoleBinding creates the RoleBinding for a GlobalRoleBinding in the namespace. An existing
// RoleBinding with incorrect content is deleted and recreated because RoleRef is immutable.
func (h *inheritedNamespacedRulesHandler) ensureRoleBinding(grb *v3.GlobalRoleBinding, ns string) error {
	desired := pkgrbac.BuildInheritedRoleBinding(grb, ns)

	existing, err := h.rbCache.Get(ns, desired.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get RoleBinding %s in namespace %s in cluster %s: %w", desired.Name, ns, h.clusterName, err)
	}

	if existing != nil {
		labelCorrect := existing.Labels != nil && existing.Labels[pkgrbac.GrbOwnerLabel] == desired.Labels[pkgrbac.GrbOwnerLabel]
		if labelCorrect && pkgrbac.IsRoleBindingContentSame(existing, desired) {
			return nil
		}
		if err := pkgrbac.DeleteNamespacedResource(ns, desired.Name, h.roleBindings); err != nil {
			return fmt.Errorf("failed to delete incorrect RoleBinding %s in namespace %s in cluster %s: %w", desired.Name, ns, h.clusterName, err)
		}
	}

	logrus.Infof("[%s] Creating RoleBinding %s in namespace %s in cluster %s", inheritedNamespacedRulesHandlerName, desired.Name, ns, h.clusterName)
	if _, err := h.roleBindings.Create(desired); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create RoleBinding %s in namespace %s in cluster %s: %w", desired.Name, ns, h.clusterName, err)
	}
	return nil
}
