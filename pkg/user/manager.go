package user

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	usertypes "github.com/rancher/rancher/pkg/user/types"
)

type TokenInput struct {
	TokenName     string
	Description   string
	Kind          string
	UserName      string
	AuthProvider  string
	TTL           *int64
	Randomize     bool
	UserPrincipal v3.Principal
	Labels        map[string]string
}

type Manager = usertypes.Manager
