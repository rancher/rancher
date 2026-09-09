package tokens

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	registry "k8s.io/apiserver/pkg/registry/generic/registry"
)

// statusDetails extracts the StatusDetails from an API error, seeing through
// wrapping via errors.As so that wrapped status errors are handled the same as
// direct ones.
func statusDetails(err error) *metav1.StatusDetails {
	var status apierrors.APIStatus
	if !errors.As(err, &status) {
		return nil
	}
	return status.Status().Details
}

// mapBackingError re-scopes an error from the backing Secret to the Token
// GroupResource, preserving the apierrors classification so clients see
// 404/409/403/422 instead of an opaque 500. Every branch rebuilds the error: a
// backing error passed through verbatim would leak the backing resource's
// identity in its details and message. Unexpected errors are logged at Error
// level; client-side errors (Invalid, BadRequest) at Warn level.
func mapBackingError(err error, resource string) error {
	switch {
	case err == nil:
		return nil
	case apierrors.IsNotFound(err):
		return apierrors.NewNotFound(GVR.GroupResource(), resource)
	case apierrors.IsConflict(err):
		return apierrors.NewConflict(GVR.GroupResource(), resource,
			errors.New(registry.OptimisticLockErrorMsg))
	case apierrors.IsAlreadyExists(err):
		return apierrors.NewAlreadyExists(GVR.GroupResource(), resource)
	case apierrors.IsForbidden(err):
		rebuilt := apierrors.NewForbidden(GVR.GroupResource(), resource, errors.New("backing store denied the request"))
		if details := statusDetails(err); details != nil && len(details.Causes) > 0 {
			rebuilt.ErrStatus.Details.Causes = details.Causes
		}
		return rebuilt
	case apierrors.IsInvalid(err):
		logrus.Warnf("tokens: invalid backing object for token %s: %v", resource, err)
		rebuilt := apierrors.NewInvalid(GVK.GroupKind(), resource, nil)
		if details := statusDetails(err); details != nil && len(details.Causes) > 0 {
			// Causes are copied as-is: their messages are already rendered by
			// the backing store, and an admission denial carries a cause with no
			// Type or Field, which field.Error would render as an "unhandled
			// error code" placeholder.
			rebuilt.ErrStatus.Details.Causes = details.Causes
			messages := make([]string, 0, len(details.Causes))
			for _, c := range details.Causes {
				if c.Field != "" {
					messages = append(messages, c.Field+": "+c.Message)
				} else {
					messages = append(messages, c.Message)
				}
			}
			rebuilt.ErrStatus.Message += ": " + strings.Join(messages, ", ")
		}
		return rebuilt
	case apierrors.IsBadRequest(err):
		logrus.Warnf("tokens: bad request on backing object for token %s: %v", resource, err)
		return apierrors.NewBadRequest(fmt.Sprintf("invalid request for token %s", resource))
	case apierrors.IsTooManyRequests(err):
		var retryAfter int32
		if details := statusDetails(err); details != nil {
			retryAfter = details.RetryAfterSeconds
		}
		logrus.Warnf("tokens: backing store throttled for token %s: %v", resource, err)
		return apierrors.NewTooManyRequests(fmt.Sprintf("too many requests for token %s", resource), int(retryAfter))
	case apierrors.IsServiceUnavailable(err):
		logrus.Warnf("tokens: backing store unavailable for token %s: %v", resource, err)
		return apierrors.NewServiceUnavailable(fmt.Sprintf("backing store unavailable for token %s", resource))
	default:
		logrus.Errorf("tokens: backing store error for token %s: %v", resource, err)
		return apierrors.NewInternalError(errors.New("error accessing backing object for token " + resource))
	}
}

// apiStatusOrInternalError returns err unchanged when it already carries an
// APIStatus (so a deliberate 4xx keeps its code) and wraps anything else as an
// InternalError.
func apiStatusOrInternalError(err error) error {
	if _, ok := err.(apierrors.APIStatus); ok {
		return err
	}
	return apierrors.NewInternalError(err)
}
