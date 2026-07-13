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
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

// SubjectAccessReview checks if a user can impersonate as another user or group
type SubjectAccessReview interface {
	// UserCanImpersonateUser checks if user can impersonate as impUser
	UserCanImpersonateUser(req *http.Request, user string, groups []string, impUser string) (bool, error)
	// UserCanImpersonateGroups checks if user can impersonate as the group
	UserCanImpersonateGroup(req *http.Request, user string, groups []string, group string) (bool, error)
	// UserCanImpersonateExtras checks if user can impersonate extras
	UserCanImpersonateExtras(req *http.Request, user string, groups []string, impExtras map[string][]string) (bool, error)
	// UserCanImpersonateServiceAccount checks if user can impersonate as the service account
	UserCanImpersonateServiceAccount(req *http.Request, user string, groups []string, sa string) (bool, error)
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
func (s subjectAccessReview) UserCanImpersonateUser(req *http.Request, user string, groups []string, impUser string) (bool, error) {
	return s.checkUserCanImpersonateUser(req.Context(), user, groups, impUser)
}

// UserCanImpersonateGroup checks if user can impersonate as the group
func (s subjectAccessReview) UserCanImpersonateGroup(req *http.Request, user string, groups []string, group string) (bool, error) {
	return s.checkUserCanImpersonateGroup(req.Context(), user, groups, group)
}

// UserCanImpersonateExtras checks if user can impersonate extras
func (s subjectAccessReview) UserCanImpersonateExtras(req *http.Request, user string, groups []string, impExtras map[string][]string) (bool, error) {
	return s.checkUserCanImpersonateExtras(req.Context(), user, groups, impExtras)
}

// UserCanImpersonateServiceAccount checks if user can impersonate as the service account
func (s subjectAccessReview) UserCanImpersonateServiceAccount(req *http.Request, user string, groups []string, sa string) (bool, error) {
	return s.checkUserCanImpersonateServiceAccount(req.Context(), user, groups, sa)
}

func (s subjectAccessReview) checkUserCanImpersonateUser(ctx context.Context, user string, groups []string, impUser string) (bool, error) {
	review := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User:   user,
			Groups: groups,
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

func (s subjectAccessReview) checkUserCanImpersonateGroup(ctx context.Context, user string, groups []string, group string) (bool, error) {
	review := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User:   user,
			Groups: groups,
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

func (s subjectAccessReview) checkUserCanImpersonateExtras(ctx context.Context, user string, groups []string, extras map[string][]string) (bool, error) {
	for name, values := range extras {
		if err := validateExtraKey(name); err != nil {
			return false, err
		}
		for _, value := range values {
			review := authv1.SubjectAccessReview{
				Spec: authv1.SubjectAccessReviewSpec{
					User:   user,
					Groups: groups,
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

func (s subjectAccessReview) checkUserCanImpersonateServiceAccount(ctx context.Context, user string, groups []string, sa string) (bool, error) {
	saNamespace, saName, err := parseServiceAccountUsername(sa)
	if err != nil {
		return false, err
	}
	review := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			User:   user,
			Groups: groups,
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb:      "impersonate",
				Resource:  "serviceaccounts",
				Namespace: saNamespace,
				Name:      saName,
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

func parseServiceAccountUsername(username string) (namespace string, name string, err error) {
	namespacedName := strings.TrimPrefix(username, serviceaccount.ServiceAccountUsernamePrefix)
	tokens := strings.Split(namespacedName, ":")
	if len(tokens) != 2 {
		return "", "", fmt.Errorf("invalid service account username format: expected system:serviceaccount:<namespace>:<name>, but got '%s'", username)
	}
	namespace = tokens[0]
	name = tokens[1]
	return namespace, name, nil
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
