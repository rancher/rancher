package common

import (
	"errors"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

var userAuthHeader = "Impersonate-User"

type UserManager interface {
	SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error)
	GetUser(apiContext *types.APIContext) string
}

func NewUserManager(mgmt *config.ManagementContext) UserManager {
	return &userManager{
		mgmt: mgmt,
	}
}

type userManager struct {
	mgmt *config.ManagementContext
}

func (u *userManager) SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error) {
	userID := u.GetUser(apiContext)
	if userID == "" {
		return nil, errors.New("user not provided")
	}

	user, err := u.mgmt.Management.Users("").Get(userID, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !slice.ContainsString(user.PrincipalIDs, principal.Name) {
		user.PrincipalIDs = append(user.PrincipalIDs, principal.Name)
		return u.mgmt.Management.Users("").Update(user)
	}
	return user, nil
}

func (u *userManager) GetUser(apiContext *types.APIContext) string {
	return apiContext.Request.Header.Get(userAuthHeader)
}
