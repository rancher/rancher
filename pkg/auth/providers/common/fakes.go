package common

import (
	"time"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	apitypes "k8s.io/apimachinery/pkg/types"
)

type FakeUserManager struct {
	HasAccess bool
}

func (m FakeUserManager) SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error) {
	panic("unimplemented")
}
func (m FakeUserManager) GetUser(apiContext *types.APIContext) string { panic("unimplemented") }

func (m FakeUserManager) EnsureUser(principalName, displayName string) (*v3.User, error) {
	panic("unimplemented")
}
func (m FakeUserManager) CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []v3.Principal) (bool, error) {
	return m.HasAccess, nil
}
func (m FakeUserManager) SetPrincipalOnCurrentUserByUserID(userID string, principal v3.Principal) (*v3.User, error) {
	panic("unimplemented")
}
func (m FakeUserManager) CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error {
	panic("unimplemented")
}
func (m FakeUserManager) GetUserByPrincipalID(principalName string) (*v3.User, error) {
	panic("unimplemented")
}

func (m FakeUserManager) GetGroupsForTokenAuthProvider(token accessor.TokenAccessor) []v3.Principal {
	panic("unimplemented")
}
func (m FakeUserManager) EnsureAndGetUserAttribute(userID string) (*v3.UserAttribute, bool, error) {
	panic("unimplemented")
}
func (m FakeUserManager) IsMemberOf(token accessor.TokenAccessor, group v3.Principal) bool {
	panic("unimplemented")
}
func (m FakeUserManager) UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []v3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error {
	panic("unimplemented")
}
