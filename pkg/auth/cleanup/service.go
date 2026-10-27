// Package cleanup defines a type that represents a cleanup routine for an auth provider.
package cleanup

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/api/secrets"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/local"
	"github.com/rancher/rancher/pkg/auth/providers/local/pbkdf2"
	"github.com/rancher/rancher/pkg/auth/providers/scim"
	ranchercontrollers "github.com/rancher/rancher/pkg/controllers"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type extTokenStore interface {
	ListForConfig(configName string) (*ext.TokenList, error)
	Delete(name string, options *metav1.DeleteOptions) error
}

// Service performs cleanup of resources associated with an auth provider.
type Service struct {
	secretsInterface wcorev1.SecretController
	secretsCache     wcorev1.SecretCache

	userClient  controllers.UserClient
	groupClient controllers.GroupClient

	authConfigsCache controllers.AuthConfigCache

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

		authConfigsCache: c.AuthConfig().Cache(),

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

	authConfigName := config.Name
	var errs []error

	if err := secrets.CleanupClientSecrets(s.secretsInterface, config); err != nil {
		errs = append(errs, fmt.Errorf("client secrets: %w", err))
	}

	if err := s.deleteGlobalRoleBindings(authConfigName); err != nil {
		errs = append(errs, fmt.Errorf("global role bindings: %w", err))
	}

	if err := s.deleteClusterRoleTemplateBindings(authConfigName); err != nil {
		errs = append(errs, fmt.Errorf("cluster role template bindings: %w", err))
	}

	if err := s.deleteProjectRoleTemplateBindings(authConfigName); err != nil {
		errs = append(errs, fmt.Errorf("project role template bindings: %w", err))
	}

	if err := s.deleteUsersAndTokens(authConfigName); err != nil {
		errs = append(errs, err)
	}

	if err := scim.Cleanup(s.secretsInterface, s.groupClient, authConfigName); err != nil {
		errs = append(errs, fmt.Errorf("SCIM secrets: %w", err))
	}

	return errors.Join(errs...)
}

// deleteUsersAndTokens deletes users first, then tokens. This ordering is required because the
// user lifecycle controller's Remove handler needs tokens to exist to discover which clusters
// have ClusterUserAttributes to clean up. If user deletion fails, token deletion is skipped
// to preserve them for the next retry.
func (s *Service) deleteUsersAndTokens(authConfigName string) error {
	if err := s.deleteUsers(authConfigName); err != nil {
		return fmt.Errorf("users: %w", err)
	}

	var errs []error

	if err := s.deleteTokens(authConfigName); err != nil {
		errs = append(errs, fmt.Errorf("tokens: %w", err))
	}

	if err := s.deleteExtTokens(authConfigName); err != nil {
		errs = append(errs, fmt.Errorf("ext tokens: %w", err))
	}

	return errors.Join(errs...)
}

func (s *Service) deleteClusterRoleTemplateBindings(authConfigName string) error {
	list, err := s.clusterRoleTemplateBindingsCache.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list cluster role template bindings: %w", err)
	}

	for _, b := range list {
		configName, err := configNameFromPrincipalNames(b.UserPrincipalName, b.GroupPrincipalName)
		if err != nil {
			logrus.Warnf("[auth cleanup] skipping cluster role template binding %s: %v", b.Namespace+"/"+b.Name, err)
			continue
		}
		if configName == authConfigName {
			err := s.clusterRoleTemplateBindingsClient.Delete(b.Namespace, b.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (s *Service) deleteGlobalRoleBindings(authConfigName string) error {
	list, err := s.globalRoleBindingsCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list global role bindings: %w", err)
	}

	for _, b := range list {
		configName, err := configNameFromPrincipalNames(b.UserPrincipalName, b.GroupPrincipalName)
		if err != nil {
			logrus.Warnf("[auth cleanup] skipping global role binding %s: %v", b.Name, err)
			continue
		}
		if configName == authConfigName {
			err := s.globalRoleBindingsClient.Delete(b.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func (s *Service) deleteProjectRoleTemplateBindings(authConfigName string) error {
	prtbs, err := s.projectRoleTemplateBindingsCache.List("", labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list project role template bindings: %w", err)
	}

	for _, b := range prtbs {
		configName, err := configNameFromPrincipalNames(b.UserPrincipalName, b.GroupPrincipalName)
		if err != nil {
			logrus.Warnf("[auth cleanup] skipping project role template binding %s: %v", b.Namespace+"/"+b.Name, err)
			continue
		}
		if configName == authConfigName {
			err := s.projectRoleTemplateBindingsClient.Delete(b.Namespace, b.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// deleteUsers removes the principals of the named AuthConfig from every user.
// A user is deleted if nothing they can log in with is left, i.e. they have
// no principals of other existing AuthConfigs and no local password. A user
// who has a local password, e.g. a local admin who set up the AuthConfig, or
// principals of other existing AuthConfigs is kept with their remaining
// principals. A user with a local password and no remaining principals is
// given their local principal, so that they can still log in locally.
// Principals of deleted AuthConfigs don't keep a user, as nobody can log in
// with them.
func (s *Service) deleteUsers(configName string) error {
	// Local principals are never removed, so local users are never deleted.
	if configName == local.Name {
		return nil
	}

	users, err := s.userClient.List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for _, u := range users.Items {
		remaining, otherConfigNames, err := principalsWithoutConfig(u.PrincipalIDs, configName)
		if err != nil {
			logrus.Warnf("[auth cleanup] skipping user %s: %v", u.Name, err)
			continue
		}
		if len(remaining) == len(u.PrincipalIDs) {
			continue
		}

		otherConfigs, err := s.anyConfigExists(otherConfigNames)
		if err != nil {
			return err
		}

		// A fully external user (who was never local) has no password.
		_, err = s.secretsCache.Get(pbkdf2.LocalUserPasswordsNamespace, u.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get user secret: %w", err)
		}
		hasPassword := u.Password != "" || err == nil

		switch {
		case !hasPassword && !otherConfigs:
			err := s.userClient.Delete(u.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		case len(remaining) == 0:
			if err := s.updateUserPrincipals(&u, []string{local.Name + "://" + u.Name}); err != nil {
				return fmt.Errorf("failed to update user principals: %w", err)
			}
		default:
			if err := s.updateUserPrincipals(&u, remaining); err != nil {
				return fmt.Errorf("failed to update user principals: %w", err)
			}
		}
	}

	return nil
}

// deleteTokens deletes the tokens issued by the named AuthConfig. Token.AuthProvider
// holds the provider type, which several AuthConfigs can share, so tokens are
// selected by the config name prefix of their user principal.
func (s *Service) deleteTokens(authConfigName string) error {
	tokens, err := s.tokensCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list tokens: %w", err)
	}

	for _, t := range tokens {
		configName, err := common.ConfigNameFromToken(t)
		if err != nil || configName != authConfigName {
			continue
		}
		err = s.tokensClient.Delete(t.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed deleting token %s while disabling auth config %s: %w", t.Name, authConfigName, err)
		}
	}

	return nil
}

func (s *Service) deleteExtTokens(authConfigName string) error {
	tokens, err := s.extTokenStore.ListForConfig(authConfigName)
	if err != nil {
		return fmt.Errorf("failed to list ext tokens: %w", err)
	}

	for i := range tokens.Items {
		if err := s.extTokenStore.Delete(tokens.Items[i].Name, &metav1.DeleteOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("failed deleting ext token %s while disabling auth config %s: %w",
				tokens.Items[i].Name, authConfigName, err)
		}
	}

	return nil
}

// updateUserPrincipals sets the user's principal IDs.
func (s *Service) updateUserPrincipals(user *v3.User, principalIDs []string) error {
	user.PrincipalIDs = principalIDs
	impClient, err := s.userClient.WithImpersonation(ranchercontrollers.WebhookImpersonation())
	if err != nil {
		return fmt.Errorf("impersonating webhook to update user principals: %w", err)
	}
	_, err = impClient.Update(user)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// anyConfigExists reports whether any of the named AuthConfigs exists.
func (s *Service) anyConfigExists(configNames []string) (bool, error) {
	for _, name := range configNames {
		_, err := s.authConfigsCache.Get(name)
		if err == nil {
			return true, nil
		}
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed to get auth config %s: %w", name, err)
		}
	}

	return false, nil
}

// principalsWithoutConfig returns the principal IDs that weren't issued by the
// named AuthConfig, and the names of the other AuthConfigs that issued any of
// them. Local and system principals belong to no AuthConfig. It returns an
// error if a principal can't be parsed.
func principalsWithoutConfig(principalIDs []string, configName string) ([]string, []string, error) {
	var remaining []string
	var otherConfigNames []string
	for _, id := range principalIDs {
		if strings.HasPrefix(id, "system://") {
			remaining = append(remaining, id)
			continue
		}
		idConfigName, _, _, err := common.SplitPrincipalID(id)
		if err != nil {
			return nil, nil, err
		}
		if idConfigName == configName {
			continue
		}
		if idConfigName != local.Name && !slices.Contains(otherConfigNames, idConfigName) {
			otherConfigNames = append(otherConfigNames, idConfigName)
		}
		remaining = append(remaining, id)
	}

	return remaining, otherConfigNames, nil
}

// configNameFromPrincipalNames returns the name of the AuthConfig that issued
// the first principal in names that was issued by one. Empty names, local
// principals and system principals, which belong to no AuthConfig, are
// skipped. It returns "" if no principal was issued by an AuthConfig, and an
// error if a principal can't be parsed.
func configNameFromPrincipalNames(names ...string) (string, error) {
	for _, name := range names {
		if name == "" || strings.HasPrefix(name, "system://") {
			continue
		}
		configName, _, _, err := common.SplitPrincipalID(name)
		if err != nil {
			return "", err
		}
		if configName != local.Name {
			return configName, nil
		}
	}

	return "", nil
}
