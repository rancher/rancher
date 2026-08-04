package kubeconfig

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
		}
		return rebuilt
	case apierrors.IsInvalid(err):
		logrus.Warnf("invalid backing object for kubeconfig %s: %v", resource, err)
		var errs field.ErrorList
		if details := statusDetails(err); details != nil {
			for _, c := range details.Causes {
				errs = append(errs, &field.Error{
					Type:   field.ErrorType(c.Type),
					Field:  c.Field,
					Detail: c.Message,
				})
			}
		}
		return apierrors.NewInvalid(schema.GroupKind{Group: gvr.Group, Kind: Kind}, resource, errs)
	case apierrors.IsBadRequest(err):
		logrus.Warnf("bad request on backing object for kubeconfig %s: %v", resource, err)
		return apierrors.NewBadRequest(fmt.Sprintf("invalid request for kubeconfig %s", resource))
	case apierrors.IsTooManyRequests(err):
		var retryAfter int32
		if details := statusDetails(err); details != nil {
			retryAfter = details.RetryAfterSeconds
		}
		logrus.Warnf("backing store throttled for kubeconfig %s: %v", resource, err)
		return apierrors.NewTooManyRequests(fmt.Sprintf("too many requests for kubeconfig %s", resource), int(retryAfter))
	case apierrors.IsServiceUnavailable(err):
		logrus.Warnf("backing store unavailable for kubeconfig %s: %v", resource, err)
		return apierrors.NewServiceUnavailable(fmt.Sprintf("backing store unavailable for kubeconfig %s", resource))
	default:
		logrus.Errorf("backing store error for kubeconfig %s: %v", resource, err)
		return apierrors.NewInternalError(errors.New("error accessing backing object for kubeconfig " + resource))
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
