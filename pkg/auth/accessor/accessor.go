package accessor

// XXX TODO AK -- marker of code modified for ext token support -- NEW FILE

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
