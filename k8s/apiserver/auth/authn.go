package auth

import (
	"net/http"

	"github.com/rancher/norman/types/slice"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type allAuthx struct {
}

func NewAuthentication() authenticator.Request {
	return &group.AuthenticatedGroupAdder{
		Authenticator: &allAuthx{},
	}
}

func NewAuthorizer(next authorizer.Authorizer) authorizer.Authorizer {
	a := &allAuthx{}
	return a.Authorizer(next)
}

func (a *allAuthx) AuthenticateRequest(req *http.Request) (user.Info, bool, error) {
	return &user.DefaultInfo{
		Name:   "admin",
		Groups: []string{"system:masters"},
	}, true, nil
}

func (a *allAuthx) Authorizer(next authorizer.Authorizer) authorizer.Authorizer {
	return authorizer.AuthorizerFunc(func(a authorizer.Attributes) (bool, string, error) {
		if a.GetUser() != nil && slice.ContainsString(a.GetUser().GetGroups(), "system:masters") {
			return true, "", nil
		}
		if next == nil {
			return true, "", nil
		}
		return next.Authorize(a)
	})
}
