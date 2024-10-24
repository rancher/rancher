package accessor

import v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

// TokenAccessor is an interface hiding the details of the different Token
// structures from the parts of Rancher having to work with both. The structures
// currently implementing this interface are Norman in file
// `pkg/apis/management.cattle.io/v3/authn_types.go`, and imperative in file
// `pkg/apis/ext.cattle.io/v1/types.go`
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
