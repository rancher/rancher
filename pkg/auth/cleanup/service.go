// Package cleanup defines a type that represents a cleanup routine for an auth provider.
package cleanup

import (
	"errors"
	"fmt"
	"strings"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/api/secrets"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	"github.com/rancher/rancher/pkg/auth/providers/scim"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
)

type extTokenStore interface {
	ListForProvider(provider string) (*ext.TokenList, error)
	Delete(name string, options *metav1.DeleteOptions) error
}

// Service performs cleanup of resources associated with an auth provider.
type Service struct {
	secretsInterface wcorev1.SecretController
	secretsCache     wcorev1.SecretCache

	userClient  controllers.UserClient
	groupClient controllers.GroupClient

	clusterRoleTemplateBindingsCache  controllers.ClusterRoleTemplateBindingCache
	clusterRoleTemplateBindingsClient controllers.ClusterRoleTemplateBindingClient

	globalRoleBindingsCache  controllers.GlobalRoleBindingCache
	globalRoleBindingsClient controllers.GlobalRoleBindingClient

	projectRoleTemplateBindingsCache  controllers.ProjectRoleTemplateBindingCache
	projectRoleTemplateBindingsClient controllers.ProjectRoleTemplateBindingClient

	tokensCache  controllers.TokenCache
	tokensClient controllers.TokenClient

	extTokenStore extTokenStore
}

// NewCleanupService creates and returns a new auth provider cleanup service.
func NewCleanupService(secretsInterface wcorev1.SecretController, c controllers.Interface, extTokens extTokenStore) *Service {
	return &Service{
		secretsInterface: secretsInterface,
		secretsCache:     secretsInterface.Cache(),

		userClient:  c.User(),
		groupClient: c.Group(),

		clusterRoleTemplateBindingsCache:  c.ClusterRoleTemplateBinding().Cache(),
		clusterRoleTemplateBindingsClient: c.ClusterRoleTemplateBinding(),

		projectRoleTemplateBindingsCache:  c.ProjectRoleTemplateBinding().Cache(),
		projectRoleTemplateBindingsClient: c.ProjectRoleTemplateBinding(),

		globalRoleBindingsCache:  c.GlobalRoleBinding().Cache(),
		globalRoleBindingsClient: c.GlobalRoleBinding(),

		tokensCache:  c.Token().Cache(),
		tokensClient: c.Token(),

		extTokenStore: extTokens,
	}
}

// Run takes an auth config and ensures that any resources associated with its auth provider,
// such as secrets, CRTBs, PRTBs, GRBs, users, tokens, are deleted.
// Independent steps are best-effort: each runs regardless of earlier failures, and all errors
// are collected and returned together.
func (s *Service) Run(config *v3.AuthConfig) error {
	if config == nil {
		return fmt.Errorf("cannot clean up resources: auth config is nil")
	}

	provider := config.Name
	var errs []error

	if err := secrets.CleanupClientSecrets(s.secretsInterface, config); err != nil {
		errs = append(errs, fmt.Errorf("client secrets: %w", err))
	}

	if err := s.deleteGlobalRoleBindings(provider); err != nil {
		errs = append(errs, fmt.Errorf("global role bindings: %w", err))
	}

	if err := s.deleteClusterRoleTemplateBindings(provider); err != nil {
		errs = append(errs, fmt.Errorf("cluster role template bindings: %w", err))
	}

	if err := s.deleteProjectRoleTemplateBindings(provider); err != nil {
		errs = append(errs, fmt.Errorf("project role template bindings: %w", err))
	}

	if err := s.deleteUsersAndTokens(provider); err != nil {
		errs = append(errs, err)
	}

	if err := scim.Cleanup(s.secretsInterface, s.groupClient, provider); err != nil {
		errs = append(errs, fmt.Errorf("SCIM secrets: %w", err))
	}

	return errors.Join(errs...)
}

// deleteUsersAndTokens deletes users first, then tokens. This ordering is required because the
// user lifecycle controller's Remove handler needs tokens to exist to discover which clusters
// have ClusterUserAttributes to clean up. If user deletion fails, token deletion is skipped
// to preserve them for the next retry.
func (s *Service) deleteUsersAndTokens(provider string) error {
	if err := s.deleteUsers(provider); err != nil {
		return fmt.Errorf("users: %w", err)
	}

	var errs []error

	if err := s.deleteTokens(provider); err != nil {
		errs = append(errs, fmt.Errorf("tokens: %w", err))
	}

	if err := s.deleteExtTokens(provider); err != nil {
		errs = append(errs, fmt.Errorf("ext tokens: %w", err))
	}

	return errors.Join(errs...)
}

func (s *Service) deleteClusterRoleTemplateBindings(provider string) error {
	list, err := s.clusterRoleTemplateBindingsCache.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list cluster role template bindings: %w", err)
	}

	for _, b := range list {
		if getProvidersFromPrincipalNames(b.UserPrincipalName, b.GroupPrincipalName).Has(provider) {
			err := s.clusterRoleTemplateBindingsClient.Delete(b.Namespace, b.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (s *Service) deleteGlobalRoleBindings(provider string) error {
	list, err := s.globalRoleBindingsCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list global role bindings: %w", err)
	}

	for _, b := range list {
		if getProvidersFromPrincipalNames(b.GroupPrincipalName).Has(provider) {
			err := s.globalRoleBindingsClient.Delete(b.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (s *Service) deleteProjectRoleTemplateBindings(provider string) error {
	prtbs, err := s.projectRoleTemplateBindingsCache.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list project role template bindings: %w", err)
	}

	for _, b := range prtbs {
		if getProvidersFromPrincipalNames(b.UserPrincipalName, b.GroupPrincipalName).Has(provider) {
			err := s.projectRoleTemplateBindingsClient.Delete(b.Namespace, b.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// deleteUsers deletes all external users for the given provider,
// who never were local users. It does not delete external users who were local before the provider had been set up.
// The method only removes the external principal IDs from those users.
// External users are those who have multiple principal IDs associated with them.
// A local admin (not necessarily the default admin) who had set up the provider will have two principal IDs,
// but will also have a password.
// This is how Rancher distinguishes fully external users from those who are external, too, but were once local.
func (s *Service) deleteUsers(provider string) error {
	users, err := s.userClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for _, u := range users.Items {
		providers := getProvidersFromPrincipalNames(u.PrincipalIDs...)
		if !providers.Has(provider) || providers.Len() > 1 {
			continue
		}
		// A fully external user (who was never local) has no password.
		_, err := s.secretsCache.Get(pbkdf2.LocalUserPasswordsNamespace, u.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get user secret: %w", err)
		}
		if u.Password == "" && apierrors.IsNotFound(err) {
			err := s.userClient.Delete(u.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		} else {
			if err := s.resetLocalUser(&u); err != nil {
				return fmt.Errorf("failed to reset local user: %w", err)
			}
		}
	}

	return nil
}

func (s *Service) deleteTokens(provider string) error {
	tokens, err := s.tokensCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list tokens: %w", err)
	}

	for _, t := range tokens {
		if t.AuthProvider == provider {
			err := s.tokensClient.Delete(t.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed deleting token %s while disabling auth provider %s: %w", t.Name, provider, err)
			}
		}
	}

	return nil
}

func (s *Service) deleteExtTokens(provider string) error {
	tokens, err := s.extTokenStore.ListForProvider(provider)
	if err != nil {
		return fmt.Errorf("failed to list ext tokens: %w", err)
	}

	for i := range tokens.Items {
		if err := s.extTokenStore.Delete(tokens.Items[i].Name, &metav1.DeleteOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed deleting ext token %s while disabling auth provider %s: %w",
				tokens.Items[i].Name, provider, err)
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

// getProvidersFromPrincipalNames returns the set of non-local provider names extracted from
// the given principal ID strings. Each principal has the form "<provider>_<type>://<id>",
// where <type> is "user" or "group". Local principals are excluded from the result.
func getProvidersFromPrincipalNames(names ...string) sets.Set[string] {
	providers := sets.New[string]()
	for _, name := range names {
		parts := strings.Split(name, "_")
		if len(parts) > 0 && parts[0] != "" && !strings.HasPrefix(parts[0], "local") {
			providers.Insert(parts[0])
		}
	}
	return providers
}
