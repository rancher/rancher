package dashboard

import (
	"context"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	BootstrapAdminConfig   = "admincreated"
	DefaultAdminLabelKey   = "authz.management.cattle.io/bootstrapping"
	DefaultAdminLabelValue = "admin-user"
)

var DefaultAdminLabel = map[string]string{DefaultAdminLabelKey: DefaultAdminLabelValue}

// bootstrapAdmin checks if the BootstrapAdminConfig exists, if it does this indicates rancher has
// already created the admin user and should not attempt it again. Otherwise attempt to create the admin.
func BootstrapAdmin(management *wrangler.Context, createClusterRoleBinding bool) (string, error) {
	if settings.NoDefaultAdmin.Get() == "true" {
		return "", nil
	}
	var adminName string

	set := labels.Set(DefaultAdminLabel)
	admins, err := management.Mgmt.User().List(v1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return "", err
	}

	if len(admins.Items) > 0 {
		adminName = admins.Items[0].Name
	}

	if _, err := management.K8s.CoreV1().ConfigMaps(namespace.System).Get(context.TODO(), BootstrapAdminConfig, v1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Warnf("Unable to determine if admin user already created: %v", err)
			return "", nil
		}
	} else {
		// config map already exists, nothing to do
		return adminName, nil
	}

	users, err := management.Mgmt.User().List(v1.ListOptions{})
	if err != nil {
		return "", err
	}

	if len(users.Items) == 0 {
		// Config map does not exist and no users, attempt to create the default admin user
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		admin, err := management.Mgmt.User().Create(&v3.User{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "user-",
				Labels:       DefaultAdminLabel,
			},
			DisplayName:        "Default Admin",
			Username:           "admin",
			Password:           string(hash),
			MustChangePassword: true,
		})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return "", errors.Wrap(err, "can not ensure admin user exists")
		}
		adminName = admin.Name

		bindings, err := management.Mgmt.GlobalRoleBinding().List(v1.ListOptions{LabelSelector: set.String()})
		if err != nil {
			logrus.Warnf("Failed to create default admin global role binding: %v", err)
			bindings = &v3.GlobalRoleBindingList{}
		}
		if len(bindings.Items) == 0 {
			adminRole := "admin"
			if settings.RestrictedDefaultAdmin.Get() == "true" {
				adminRole = "restricted-admin"
			}
			_, err = management.Mgmt.GlobalRoleBinding().Create(
				&v3.GlobalRoleBinding{
					ObjectMeta: v1.ObjectMeta{
						GenerateName: "globalrolebinding-",
						Labels:       DefaultAdminLabel,
					},
					UserName:       adminName,
					GlobalRoleName: adminRole,
				})
			if err != nil {
				logrus.Warnf("Failed to create default admin global role binding: %v", err)
			} else {
				logrus.Info("Created default admin user and binding")
			}
		}

		if createClusterRoleBinding && settings.RestrictedDefaultAdmin.Get() != "true" {
			users, err := management.Mgmt.User().List(v1.ListOptions{
				LabelSelector: set.String(),
			})

			bindings, err := management.RBAC.ClusterRoleBinding().List(v1.ListOptions{LabelSelector: set.String()})
			if err != nil {
				return "", err
			}
			if len(bindings.Items) == 0 && len(users.Items) > 0 {
				_, err = management.RBAC.ClusterRoleBinding().Create(
					&rbacv1.ClusterRoleBinding{
						ObjectMeta: v1.ObjectMeta{
							GenerateName: "default-admin-",
							Labels:       DefaultAdminLabel,
							OwnerReferences: []v1.OwnerReference{
								{
									APIVersion: "management.cattle.io/v3",
									Kind:       "User",
									Name:       users.Items[0].Name,
									UID:        users.Items[0].UID,
								},
							},
						},
						Subjects: []rbacv1.Subject{
							{
								Kind:     "User",
								APIGroup: rbacv1.GroupName,
								Name:     users.Items[0].Name,
							},
						},
						RoleRef: rbacv1.RoleRef{
							APIGroup: rbacv1.GroupName,
							Kind:     "ClusterRole",
							Name:     "cluster-admin",
						},
					})
				if err != nil {
					logrus.Warnf("Failed to create default admin global role binding: %v", err)
				} else {
					logrus.Info("Created default admin user and binding")
				}
			}
		}
	}

	adminConfigMap := corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      BootstrapAdminConfig,
			Namespace: namespace.System,
		},
	}

	_, err = management.K8s.CoreV1().ConfigMaps(namespace.System).Create(context.TODO(), &adminConfigMap, v1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logrus.Warnf("Error creating admin config map: %v", err)
		}

	}
	return adminName, nil
}
