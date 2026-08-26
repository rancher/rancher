package rbac

import (
	"errors"
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	wrbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	wname "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	roleBindings wrbacv1.RoleBindingClient
	rbCache      wrbacv1.RoleBindingCache
	clusterName  string
}

func newInheritedNamespacedRulesHandler(workload *config.UserContext) *inheritedNamespacedRulesHandler {
	return &inheritedNamespacedRulesHandler{
		grCache:      workload.Management.Wrangler.Mgmt.GlobalRole().Cache(),
		grbCache:     workload.Management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
		roles:        workload.RBACw.Role(),
		roleBindings: workload.RBACw.RoleBinding(),
		rbCache:      workload.RBACw.RoleBinding().Cache(),
		clusterName:  workload.ClusterName,
	}
}

func (h *inheritedNamespacedRulesHandler) onNamespaceChange(_ string, ns *corev1.Namespace) (*corev1.Namespace, error) {
	if ns == nil || ns.DeletionTimestamp != nil || ns.Status.Phase == corev1.NamespaceTerminating {
		return ns, nil
	}

	grs, err := h.grCache.GetByIndex(pkgrbac.GRDownstreamNSIndex, ns.Name)
	if err != nil {
		return ns, fmt.Errorf("failed to list GlobalRoles for namespace %s: %w", ns.Name, err)
	}

	var returnError error
	for _, gr := range grs {
		rules, ok := gr.InheritedNamespacedRules[ns.Name]
		if !ok {
			continue
		}
		if err := h.ensureRole(gr, ns.Name, rules); err != nil {
			returnError = errors.Join(returnError, err)
			continue
		}

		grbs, err := h.grbCache.GetByIndex(pkgrbac.GRBGlobalRoleIndex, gr.Name)
		if err != nil {
			returnError = errors.Join(returnError, fmt.Errorf("failed to list GlobalRoleBindings for GlobalRole %s: %w", gr.Name, err))
			continue
		}
		for _, grb := range grbs {
			if err := h.ensureRoleBinding(gr, grb, ns.Name); err != nil {
				returnError = errors.Join(returnError, err)
			}
		}
	}

	return ns, returnError
}

// ensureRole creates or updates the Role for a GlobalRole's inherited rules in the namespace,
// with the same name, labels and rules as the leader-side GlobalRole controller.
func (h *inheritedNamespacedRulesHandler) ensureRole(gr *v3.GlobalRole, ns string, rules []rbacv1.PolicyRule) error {
	desired := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wname.SafeConcatName(gr.Name, ns),
			Namespace: ns,
			Labels: map[string]string{
				pkgrbac.GrOwnerLabel: wname.SafeConcatName(gr.Name),
			},
		},
		Rules: rules,
	}

	_, err := pkgrbac.CreateOrUpdateNamespacedResource(desired, h.roles, func(existing, desired *rbacv1.Role) bool {
		return equality.Semantic.DeepEqual(desired.Rules, existing.Rules) &&
			equality.Semantic.DeepEqual(desired.Labels, existing.Labels)
	})
	if err != nil {
		return fmt.Errorf("failed to ensure Role for GlobalRole %s in namespace %s in cluster %s: %w", gr.Name, ns, h.clusterName, err)
	}
	return nil
}

// ensureRoleBinding creates the RoleBinding for a GlobalRoleBinding in the namespace, with the
// same name, labels and content as the leader-side GlobalRoleBinding controller. An existing
// RoleBinding with incorrect content is deleted and recreated because RoleRef is immutable.
func (h *inheritedNamespacedRulesHandler) ensureRoleBinding(gr *v3.GlobalRole, grb *v3.GlobalRoleBinding, ns string) error {
	grbName := wname.SafeConcatName(grb.Name)
	desired := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wname.SafeConcatName(grbName, ns),
			Namespace: ns,
			Labels: map[string]string{
				pkgrbac.GrbOwnerLabel: grbName,
			},
		},
		Subjects: []rbacv1.Subject{pkgrbac.GetGRBSubject(grb)},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     wname.SafeConcatName(wname.SafeConcatName(gr.Name), ns),
		},
	}

	existing, err := h.rbCache.Get(ns, desired.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get RoleBinding %s in namespace %s in cluster %s: %w", desired.Name, ns, h.clusterName, err)
	}

	if existing != nil {
		labelCorrect := existing.Labels != nil && existing.Labels[pkgrbac.GrbOwnerLabel] == grbName
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
