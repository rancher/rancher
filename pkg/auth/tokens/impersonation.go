package tokens

import (
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
)

const (
	ImpersonationSessionAnnotation = "authn.management.cattle.io/impersonation-session"
	ImpersonatorTokenAnnotation    = "authn.management.cattle.io/impersonator-token-id"
	ImpersonatorUserAnnotation     = "authn.management.cattle.io/impersonator-user-id"
)

// BindImpersonationSession records the identities of the original session and
// its owner without persisting the bearer token secret.
func BindImpersonationSession(effective *apiv3.Token, original accessor.TokenAccessor) {
	if effective.Annotations == nil {
		effective.Annotations = map[string]string{}
	}
	effective.Annotations[ImpersonationSessionAnnotation] = "true"
	effective.Annotations[ImpersonatorTokenAnnotation] = original.GetFullName()
	effective.Annotations[ImpersonatorUserAnnotation] = original.GetUserID()
}

// IsImpersonationSessionBound verifies that effective was created from the
// supplied original session.
func IsImpersonationSessionBound(effective, original accessor.TokenAccessor) bool {
	if effective == nil || original == nil {
		return false
	}
	annotated, ok := effective.(interface{ GetAnnotations() map[string]string })
	if !ok {
		return false
	}
	annotations := annotated.GetAnnotations()
	return annotations[ImpersonationSessionAnnotation] == "true" &&
		annotations[ImpersonatorTokenAnnotation] == original.GetFullName() &&
		annotations[ImpersonatorUserAnnotation] == original.GetUserID()
}
