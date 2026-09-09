package kubeconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// mapBackingError re-scopes an error from a backing object (ConfigMap or token)
// to the Kubeconfig GroupResource, preserving the apierrors classification so
// clients see 404/409/403/422 instead of an opaque 500. Every branch rebuilds
// the error: a backing error passed through verbatim would leak the backing
// resource's identity in its details and message. Unexpected errors are logged
// at Error level; client-side errors (Invalid, BadRequest) at Warn level.
func mapBackingError(err error, resource string) error {
	switch {
	case err == nil:
		return nil
	case apierrors.IsNotFound(err):
		return apierrors.NewNotFound(gvr.GroupResource(), resource)
	case apierrors.IsConflict(err):
		return apierrors.NewConflict(gvr.GroupResource(), resource,
			errors.New(registry.OptimisticLockErrorMsg))
	case apierrors.IsAlreadyExists(err):
		return apierrors.NewAlreadyExists(gvr.GroupResource(), resource)
	case apierrors.IsForbidden(err):
		rebuilt := apierrors.NewForbidden(gvr.GroupResource(), resource, errors.New("backing store denied the request"))
		if details := statusDetails(err); details != nil && len(details.Causes) > 0 {
			rebuilt.ErrStatus.Details.Causes = details.Causes
			rebuilt.ErrStatus.Message += ": " + causeMessages(details.Causes)
		}
		return rebuilt
	case apierrors.IsInvalid(err):
		logrus.Warnf("kubeconfig: invalid backing object for kubeconfig %s: %v", resource, err)
		rebuilt := apierrors.NewInvalid(schema.GroupKind{Group: gvr.Group, Kind: Kind}, resource, nil)
		if details := statusDetails(err); details != nil && len(details.Causes) > 0 {
			// Causes are copied as-is: their messages are already rendered by
			// the backing store, and an admission denial carries a cause with no
			// Type or Field, which field.Error would render as an "unhandled
			// error code" placeholder.
			rebuilt.ErrStatus.Details.Causes = details.Causes
			rebuilt.ErrStatus.Message += ": " + causeMessages(details.Causes)
		}
		return rebuilt
	case apierrors.IsBadRequest(err):
		logrus.Warnf("kubeconfig: bad request on backing object for kubeconfig %s: %v", resource, err)
		return apierrors.NewBadRequest(fmt.Sprintf("invalid request for kubeconfig %s", resource))
	case apierrors.IsTooManyRequests(err):
		var retryAfter int32
		if details := statusDetails(err); details != nil {
			retryAfter = details.RetryAfterSeconds
		}
		logrus.Warnf("kubeconfig: backing store throttled for kubeconfig %s: %v", resource, err)
		return apierrors.NewTooManyRequests(fmt.Sprintf("too many requests for kubeconfig %s", resource), int(retryAfter))
	case apierrors.IsServiceUnavailable(err):
		logrus.Warnf("kubeconfig: backing store unavailable for kubeconfig %s: %v", resource, err)
		return apierrors.NewServiceUnavailable(fmt.Sprintf("backing store unavailable for kubeconfig %s", resource))
	default:
		logrus.Errorf("kubeconfig: backing store error for kubeconfig %s: %v", resource, err)
		return apierrors.NewInternalError(errors.New("error accessing backing object for kubeconfig " + resource))
	}
}

// asAPIStatus returns the status error carried anywhere in err's chain,
// unwrapped, or nil when there is none. Unwrapping matters: the apiserver
// derives the HTTP code with a type switch and would report a wrapped status
// error as a 500.
func asAPIStatus(err error) error {
	var status apierrors.APIStatus
	if !errors.As(err, &status) {
		return nil
	}
	if statusErr, ok := status.(error); ok {
		return statusErr
	}
	return nil
}

// apiStatusOrInternalError returns the status error carried by err so a
// deliberate 4xx keeps its code, and wraps anything else as an InternalError.
func apiStatusOrInternalError(err error) error {
	if statusErr := asAPIStatus(err); statusErr != nil {
		return statusErr
	}
	return apierrors.NewInternalError(err)
}

// causeMessages renders status causes for a message, prefixing each with its
// field when it has one.
func causeMessages(causes []metav1.StatusCause) string {
	messages := make([]string, 0, len(causes))
	for _, c := range causes {
		if c.Field != "" {
			messages = append(messages, c.Field+": "+c.Message)
		} else {
			messages = append(messages, c.Message)
		}
	}
	return strings.Join(messages, ", ")
}
