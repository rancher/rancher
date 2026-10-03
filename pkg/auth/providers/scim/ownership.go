package scim

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// userOwnedBy reports whether the provider's SCIM endpoints may act on the user.
// The user must hold a principal of the provider and must not be a system user or the default
// admin. A local user who linked the provider through test-and-enable is owned by it, with or
// without a password. The local://<name> principal every user carries isn't a principal of the
// provider.
func userOwnedBy(provider string, user *v3.User) bool {
	if user.IsSystem() || user.IsDefaultAdmin() {
		return false
	}

	prefix := userPrincipalName(provider, "")
	return slices.ContainsFunc(user.PrincipalIDs, func(id string) bool { return strings.HasPrefix(id, prefix) })
}

// groupOwnedBy reports whether the group belongs to the provider.
func groupOwnedBy(provider string, group *v3.Group) bool {
	return group.Labels[authProviderLabel] == provider
}

// getOwnedUser returns the user with the given ID.
// It returns a not found error when the user doesn't exist or isn't owned by the provider.
func (s *SCIMServer) getOwnedUser(provider, id string) (*v3.User, error) {
	user, err := s.userCache.Get(id)
	if err != nil {
		return nil, err
	}

	if !userOwnedBy(provider, user) {
		logrus.Debugf("scim::getOwnedUser: user %s isn't owned by provider %s", id, provider)
		return nil, apierrors.NewNotFound(v3.Resource("user"), id)
	}

	return user, nil
}

// getOwnedGroup returns the group with the given ID.
// It returns a not found error when the group doesn't exist or isn't owned by the provider.
func (s *SCIMServer) getOwnedGroup(provider, id string) (*v3.Group, error) {
	group, err := s.groupsCache.Get(id)
	if err != nil {
		return nil, err
	}

	if !groupOwnedBy(provider, group) {
		logrus.Debugf("scim::getOwnedGroup: group %s isn't owned by provider %s", id, provider)
		return nil, apierrors.NewNotFound(v3.Resource("group"), id)
	}

	return group, nil
}

// notManagedConflict returns the error for a user who holds the provider's principal but isn't
// owned by the provider.
func notManagedConflict(name string) *Error {
	return NewError(http.StatusConflict, fmt.Sprintf("User %s can't be managed by SCIM", name))
}
