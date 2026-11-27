package cloudcredential

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	registry "k8s.io/apiserver/pkg/registry/generic/registry"
)

func statusDetails(err error) *metav1.StatusDetails {
	var status apierrors.APIStatus
	if !errors.As(err, &status) {
		return nil
	}
	return status.Status().Details
}

func mapBackingError(err error, resource string) error {
	switch {
	case err == nil:
		return nil
	case apierrors.IsNotFound(err):
		return apierrors.NewNotFound(GR, resource)
	case apierrors.IsConflict(err):
		return apierrors.NewConflict(GR, resource, errors.New(registry.OptimisticLockErrorMsg))
	case apierrors.IsAlreadyExists(err):
		return apierrors.NewAlreadyExists(GR, resource)
	case apierrors.IsForbidden(err):
		rebuilt := apierrors.NewForbidden(GR, resource, errors.New("backing store denied the request"))
		if details := statusDetails(err); details != nil && len(details.Causes) > 0 {
			rebuilt.ErrStatus.Details.Causes = details.Causes
		}
		return rebuilt
	case apierrors.IsInvalid(err):
		logrus.Warnf("invalid backing Secret for cloud credential %s: %v", resource, err)
		var causes field.ErrorList
		if details := statusDetails(err); details != nil {
			for _, cause := range details.Causes {
				causes = append(causes, &field.Error{
					Type:   field.ErrorType(cause.Type),
					Field:  cause.Field,
					Detail: cause.Message,
				})
			}
		}
		return apierrors.NewInvalid(schema.GroupKind{Group: GV.Group, Kind: GVK.Kind}, resource, causes)
	case apierrors.IsBadRequest(err):
		logrus.Warnf("bad request on backing Secret for cloud credential %s: %v", resource, err)
		return apierrors.NewBadRequest(fmt.Sprintf("invalid request for cloud credential %s", resource))
	case apierrors.IsTooManyRequests(err):
		retryAfter := 0
		if details := statusDetails(err); details != nil {
			retryAfter = int(details.RetryAfterSeconds)
		}
		logrus.Warnf("backing Secret throttled for cloud credential %s: %v", resource, err)
		return apierrors.NewTooManyRequests(fmt.Sprintf("too many requests for cloud credential %s", resource), retryAfter)
	case apierrors.IsServiceUnavailable(err):
		logrus.Warnf("backing Secret unavailable for cloud credential %s: %v", resource, err)
		return apierrors.NewServiceUnavailable(fmt.Sprintf("backing Secret unavailable for cloud credential %s", resource))
	default:
		logrus.Errorf("backing Secret error for cloud credential %s: %v", resource, err)
		return apierrors.NewInternalError(errors.New("error accessing backing Secret for cloud credential " + resource))
	}
}

func apiStatusOrInternalError(err error) error {
	if err == nil {
		return nil
	}
	var status apierrors.APIStatus
	if errors.As(err, &status) {
		return err
	}
	return apierrors.NewInternalError(err)
}

func isAPIStatus(err error) bool {
	var status apierrors.APIStatus
	return errors.As(err, &status)
}

func checkPreconditions(preconditions *metav1.Preconditions, name, resourceVersion string, uids ...types.UID) error {
	if preconditions == nil {
		return nil
	}
	if preconditions.UID != nil {
		matched := false
		for _, uid := range uids {
			if *preconditions.UID == uid {
				matched = true
				break
			}
		}
		if !matched {
			var recordUID types.UID
			if len(uids) > 0 {
				recordUID = uids[0]
			}
			return apierrors.NewConflict(GR, name, fmt.Errorf(
				"the UID in the precondition (%s) does not match the UID in record (%s)",
				*preconditions.UID, recordUID))
		}
	}
	if preconditions.ResourceVersion != nil && *preconditions.ResourceVersion != resourceVersion {
		return apierrors.NewConflict(GR, name, fmt.Errorf(
			"the ResourceVersion in the precondition (%s) does not match the ResourceVersion in record (%s)",
			*preconditions.ResourceVersion, resourceVersion))
	}
	return nil
}
