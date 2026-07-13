package sar

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// SubjectAccessReview checks if a user can impersonate as another user or group
type SubjectAccessReview interface {
	// UserCanImpersonateUser checks if user can impersonate as impUser
	UserCanImpersonateUser(req *http.Request, user, impUser string) (bool, error)
	// UserCanImpersonateGroups checks if user can impersonate as the group
	UserCanImpersonateGroup(req *http.Request, user string, group string) (bool, error)
	// UserCanImpersonateExtras checks if user can impersonate extras
	UserCanImpersonateExtras(req *http.Request, user string, impExtras map[string][]string) (bool, error)
	// UserCanImpersonateServiceAccount checks if user can impersonate as the service account
	UserCanImpersonateServiceAccount(req *http.Request, user string, sa string) (bool, error)
}

// subjectAccessReview implements SubjectAccessReview interface.
type subjectAccessReview struct {
	sarClient authorizationv1.SubjectAccessReviewInterface
}

// NewSubjectAccessReview creates a new SubjectAccessReview with the given
// SubjectAccessReviewInterface client for performing SAR checks.
func NewSubjectAccessReview(sarClient authorizationv1.SubjectAccessReviewInterface) SubjectAccessReview {
	return subjectAccessReview{
		sarClient: sarClient,
	}
}

// UserCanImpersonateUser checks if user can impersonate as impUser
func (sar subjectAccessReview) UserCanImpersonateUser(req *http.Request, user, impUser string) (bool, error) {
	return sar.checkUserCanImpersonateUser(req.Context(), user, impUser)
}

// UserCanImpersonateServiceAccount checks if user can impersonate as the service account
func (sar subjectAccessReview) UserCanImpersonateServiceAccount(req *http.Request, user string, sa string) (bool, error) {
	return sar.checkUserCanImpersonateServiceAccount(req.Context(), user, sa)
}

// UserCanImpersonateGroup checks if user can impersonate as the group
func (s subjectAccessReview) UserCanImpersonateGroup(req *http.Request, user string, group string) (bool, error) {
	return s.checkUserCanImpersonateGroup(req.Context(), user, group)
}

// UserCanImpersonateExtras checks if user can impersonate extras
func (s subjectAccessReview) UserCanImpersonateExtras(req *http.Request, user string, impExtras map[string][]string) (bool, error) {
	return s.checkUserCanImpersonateExtras(req.Context(), user, impExtras)
}

func (s subjectAccessReview) checkUserCanImpersonateUser(ctx context.Context, user, impUser string) (bool, error) {
	review := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     "impersonate",
				Resource: "users",
				Name:     impUser,
			},
		},
	}

	result, err := s.sarClient.Create(ctx, &review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	logrus.Debugf("Impersonate check result: %v", result)
	return result.Status.Allowed, nil
}

func (s subjectAccessReview) checkUserCanImpersonateGroup(ctx context.Context, user, group string) (bool, error) {
	review := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     "impersonate",
				Resource: "groups",
				Name:     group,
			},
		},
	}

	result, err := s.sarClient.Create(ctx, &review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	logrus.Debugf("Impersonate check result: %v", result)

	return result.Status.Allowed, nil
}

func (s subjectAccessReview) checkUserCanImpersonateExtras(ctx context.Context, user string, extras map[string][]string) (bool, error) {
	for name, values := range extras {
		if err := validateExtraKey(name); err != nil {
			return false, err
		}
		for _, value := range values {
			review := authv1.SubjectAccessReview{
				Spec: authv1.SubjectAccessReviewSpec{
					User: user,
					ResourceAttributes: &authv1.ResourceAttributes{
						Verb:     "impersonate",
						Resource: "userextras/" + name,
						Name:     value,
					},
				},
			}

			result, err := s.sarClient.Create(ctx, &review, metav1.CreateOptions{})
			if err != nil {
				return false, err
			}
			logrus.Debugf("Impersonate check result: %v", result)
			if !result.Status.Allowed {
				return false, nil
			}
		}
	}

	return true, nil
}

func (s subjectAccessReview) checkUserCanImpersonateServiceAccount(ctx context.Context, user, sa string) (bool, error) {
	review := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:     "impersonate",
				Resource: "serviceaccounts",
				Name:     sa,
			},
		},
	}

	result, err := s.sarClient.Create(ctx, &review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	logrus.Debugf("Impersonate sa check result: %v", result)
	return result.Status.Allowed, nil
}

func validateExtraKey(name string) error {
	if name == "" {
		return errors.New("extra key must not be empty")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("extra key %q must not contain '/'", name)
	}
	return nil
}
