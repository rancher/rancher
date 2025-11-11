package user

import (
	"fmt"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PasswordHashAnnotation = "cattle.io/password-hash"
	BcryptHash             = "bcrypt"
)

type PasswordMigrator struct {
	Users   wranglerv3.UserClient
	Secrets wcorev1.SecretClient
}

func NewPasswordMigrator(wContext *wrangler.Context) *PasswordMigrator {
	return &PasswordMigrator{
		Users:   wContext.Mgmt.User(),
		Secrets: wContext.Core.Secret(),
	}
}

func (m *PasswordMigrator) MigrateAll() error {
	users, err := m.Users.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for _, user := range users.Items {
		if err := m.Migrate(&user); err != nil {
			return err
		}
	}

	return nil
}

func (m *PasswordMigrator) Migrate(user *apiv3.User) error {
	if user.Password == "" {
		return nil
	}

	passwordSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      user.Name,
			Namespace: pbkdf2.LocalUserPasswordsNamespace,
			Annotations: map[string]string{
				PasswordHashAnnotation: BcryptHash,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       user.Name,
					UID:        user.UID,
					APIVersion: "management.cattle.io/v3",
					Kind:       "User",
				},
			},
		},
		Data: map[string][]byte{
			"password": []byte(user.Password),
		},
	}

	_, err := m.Secrets.Create(passwordSecret)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create password secret: %w", err)
		}
		_, err = m.Secrets.Update(passwordSecret)
		if err != nil {
			return fmt.Errorf("failed to update password secret: %w", err)
		}
	}

	user = user.DeepCopy()
	user.Password = ""
	_, err = m.Users.Update(user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}
