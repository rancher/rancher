// Package cleanup defines a type that represents a cleanup routine for an auth provider.
package cleanup

import (
	"errors"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/api/secrets"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var errAuthConfigNil = errors.New("cannot get auth provider if its config is nil")

// Service performs cleanup of resources associated with an auth provider.
type Service struct {
	secretsInterface corev1.SecretInterface

	userCache  controllers.UserCache
	userClient controllers.UserClient

	clusterRoleTemplateBindingsCache  controllers.ClusterRoleTemplateBindingCache
	clusterRoleTemplateBindingsClient controllers.ClusterRoleTemplateBindingClient

	globalRoleBindingsCache  controllers.GlobalRoleBindingCache
	globalRoleBindingsClient controllers.GlobalRoleBindingClient

	projectRoleTemplateBindingsCache  controllers.ProjectRoleTemplateBindingCache
	projectRoleTemplateBindingsClient controllers.ProjectRoleTemplateBindingClient
}

// NewCleanupService creates and returns a new auth provider cleanup service.
func NewCleanupService(secretsInterface corev1.SecretInterface, c controllers.Interface) *Service {
	return &Service{
		secretsInterface: secretsInterface,

		userCache:  c.User().Cache(),
		userClient: c.User(),

		clusterRoleTemplateBindingsCache:  c.ClusterRoleTemplateBinding().Cache(),
		clusterRoleTemplateBindingsClient: c.ClusterRoleTemplateBinding(),

		projectRoleTemplateBindingsCache:  c.ProjectRoleTemplateBinding().Cache(),
		projectRoleTemplateBindingsClient: c.ProjectRoleTemplateBinding(),

		globalRoleBindingsCache:  c.GlobalRoleBinding().Cache(),
		globalRoleBindingsClient: c.GlobalRoleBinding(),
	}
}

// Run takes an auth config and checks if its auth provider is disabled, and ensures that any resources associated with it,
// such as secrets, CRTBs, PRTBs, GRBs, Users, are deleted.
func (s *Service) Run(config *v3.AuthConfig) error {
	if err := secrets.CleanupClientSecrets(s.secretsInterface, config); err != nil {
		return fmt.Errorf("error cleaning up resources associated with a disabled auth provider %s: %w", config.Name, err)
	}

	if err := s.deleteGlobalRoleBindings(config); err != nil {
		return fmt.Errorf("error cleaning up global role bindings: %w", err)
	}

	if err := s.deleteClusterRoleTemplateBindings(config); err != nil {
		return fmt.Errorf("error cleaning up cluster role template bindings: %w", err)
	}

	if err := s.deleteProjectRoleTemplateBindings(config); err != nil {
		return fmt.Errorf("error cleaning up project role template bindings: %w", err)
	}

	if err := s.deleteUsers(config); err != nil {
		return fmt.Errorf("error cleaning up users: %w", err)
	}

	return nil
}

func (s *Service) deleteClusterRoleTemplateBindings(config *v3.AuthConfig) error {
	if config == nil {
		return errAuthConfigNil
	}
	list, err := s.clusterRoleTemplateBindingsCache.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list cluster role template bindings: %w", err)
	}

	for _, b := range list {
		providerName := getProviderNameFromPrincipalNames(b.UserPrincipalName, b.GroupPrincipalName)
		if providerName == config.Name {
			err := s.clusterRoleTemplateBindingsClient.Delete(b.Namespace, b.Name, &v1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (s *Service) deleteGlobalRoleBindings(config *v3.AuthConfig) error {
	if config == nil {
		return errAuthConfigNil
	}
	list, err := s.globalRoleBindingsCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list global role bindings: %w", err)
	}

	for _, b := range list {
		providerName := getProviderNameFromPrincipalNames(b.GroupPrincipalName)
		if providerName == config.Name {
			err := s.globalRoleBindingsClient.Delete(b.Name, &v1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (s *Service) deleteProjectRoleTemplateBindings(config *v3.AuthConfig) error {
	if config == nil {
		return errAuthConfigNil
	}
	prtbs, err := s.projectRoleTemplateBindingsCache.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list project role template bindings: %w", err)
	}

	for _, b := range prtbs {
		providerName := getProviderNameFromPrincipalNames(b.UserPrincipalName, b.GroupPrincipalName)
		if providerName == config.Name {
			err := s.projectRoleTemplateBindingsClient.Delete(b.Namespace, b.Name, &v1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// deleteUsers deletes all external users (for the given provider specified in the config),
// who never were local users. It does not delete external users who were local before the provider had been set up.
// The method only removes the external principal IDs from those users.
// External users are those who have multiple principal IDs associated with them.
// A local admin (not necessarily the default admin) who had set up the provider will have two principal IDs,
// but will also have a password.
// This is how Rancher distinguishes fully external users from those who are external, too, but were once local.
func (s *Service) deleteUsers(config *v3.AuthConfig) error {
	if config == nil {
		return errAuthConfigNil
	}
	users, err := s.userCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for _, u := range users {
		providerName := getProviderNameFromPrincipalNames(u.PrincipalIDs...)
		if providerName == config.Name {
			// A fully external user (who was never local) has no password.
			if u.Password == "" {
				err := s.userClient.Delete(u.Name, &v1.DeleteOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
			} else {
				if err := s.resetLocalUser(u); err != nil {
					return fmt.Errorf("failed to reset local user: %w", err)
				}
			}
		}
	}

	return nil
}

// resetLocalUser takes a user and removes all its principal IDs except that of the local user.
// It updates the user so that it is effectively as it was before any auth provider had been enabled.
func (s *Service) resetLocalUser(user *v3.User) error {
	if user == nil || len(user.PrincipalIDs) < 2 {
		return nil
	}

	var localID string
	for _, id := range user.PrincipalIDs {
		if strings.HasPrefix(id, "local") {
			localID = id
			break
		}
	}

	if localID == "" {
		return nil
	}

	user.PrincipalIDs = []string{localID}
	_, err := s.userClient.Update(user)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// getProviderNameFromPrincipalNames tries to extract the provider name from any one string that represents
// a user principal or group principal.
func getProviderNameFromPrincipalNames(names ...string) string {
	for _, name := range names {
		parts := strings.Split(name, "_")
		if len(parts) > 0 && parts[0] != "" && !strings.HasPrefix(parts[0], "local") {
			return parts[0]
		}
	}
	return ""
}
