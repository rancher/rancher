package secret

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbacv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	v1 "k8s.io/api/core/v1"
	k8srbac "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		roles:             clients.RBAC.Role(),
		rolesCache:        clients.RBAC.Role().Cache(),
		roleBindings:      clients.RBAC.RoleBinding(),
		roleBindingsCache: clients.RBAC.RoleBinding().Cache(),
		secrets:           clients.Core.Secret(),
	}

	clients.Core.Secret().OnChange(ctx, "provisioning-v2-secret", h.OnSecret)
}

type handler struct {
	roles             rbacv1.RoleClient
	rolesCache        rbacv1.RoleCache
	roleBindings      rbacv1.RoleBindingClient
	roleBindingsCache rbacv1.RoleBindingCache
	secrets           wranglerv1.SecretClient
}

// OnSecret syncs a creators permissions to a provisioning cloud-credential. This is based off
// the creator ID annotation that is added in the webhook when the secret is created. A role
// and binding are created to grant the creator permissions and then the annotation is removed.
func (h *handler) OnSecret(key string, secret *v1.Secret) (*v1.Secret, error) {
	if secret == nil || secret.DeletionTimestamp != nil || secret.Type != "provisioning.cattle.io/cloud-credential" {
		return secret, nil
	}
	creatorID, ok := secret.Annotations[rbac.CreatorIDAnn]
	if !ok || creatorID == "" {
		return secret, nil
	}

	if err := h.ensureCreatorPermissions(creatorID, secret); err != nil {
		return secret, err
	}

	s := secret.DeepCopy()
	delete(s.Annotations, rbac.CreatorIDAnn)

	return h.secrets.Update(s)
}

func (h *handler) ensureCreatorPermissions(creatorID string, secret *v1.Secret) error {
	name := creatorID + "-" + secret.Name

	// The owner reference for the role and binding are tied to the secret since a
	// user is not guaranteed to be a management.cattle.io user CR
	ownerRef := metav1.OwnerReference{
		APIVersion: v1.SchemeGroupVersion.Version,
		Kind:       "Secret",
		Name:       secret.Name,
		UID:        secret.UID,
	}
	if err := h.ensureRole(secret.Namespace, name, ownerRef); err != nil {
		return err
	}

	return h.ensureBinding(secret.Namespace, name, creatorID, ownerRef)
}

func (h *handler) ensureRole(namespace, name string, ownerRef metav1.OwnerReference) error {
	_, err := h.rolesCache.Get(namespace, name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}

		role := &k8srbac.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       namespace,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Rules: []k8srbac.PolicyRule{
				{
					Verbs:         []string{"get", "update", "delete"},
					APIGroups:     []string{""},
					Resources:     []string{"secrets"},
					ResourceNames: []string{ownerRef.Name},
				},
			},
		}

		if _, err := h.roles.Create(role); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (h *handler) ensureBinding(namespace, name, creatorID string, ownerRef metav1.OwnerReference) error {
	_, err := h.roleBindingsCache.Get(namespace, name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}

		binding := &k8srbac.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       namespace,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Subjects: []k8srbac.Subject{
				{
					Kind: k8srbac.UserKind,
					Name: creatorID,
				},
			},
			RoleRef: k8srbac.RoleRef{
				Kind: "Role",
				Name: name,
			},
		}

		if _, err := h.roleBindings.Create(binding); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}
