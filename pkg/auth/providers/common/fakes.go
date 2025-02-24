package common

import (
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	apitypes "k8s.io/apimachinery/pkg/types"
)

type FakeUserManager struct {
	HasAccess bool
}

func (m FakeUserManager) SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error) {
	panic("unimplemented")
}
func (m FakeUserManager) GetUser(apiContext *types.APIContext) string       { panic("unimplemented") }
func (m FakeUserManager) EnsureToken(input user.TokenInput) (string, error) { panic("unimplemented") }
func (m FakeUserManager) EnsureClusterToken(clusterName string, input user.TokenInput) (string, error) {
	panic("unimplemented")
}
func (m FakeUserManager) DeleteToken(tokenName string) error { panic("unimplemented") }
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
func (m FakeUserManager) GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal v3.Principal) (*v3.Token, string, error) {
	panic("unimplemented")
}
