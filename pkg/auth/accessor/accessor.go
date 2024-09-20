package accessor

import v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

type TokenAccessor interface {
	GetName() string
	GetIsEnabled() bool
	GetIsDerived() bool
	GetAuthProvider() string
	GetUserID() string
	GetUserPrincipal() v3.Principal
	GetProviderInfo() map[string]string
	ObjClusterName() string
	GetGroupPrincipals() []v3.Principal
}
