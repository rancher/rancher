package sar

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
	authV1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
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

type subjectAccessReview struct {
	sarClientGetter SubjectAccessReviewClientGetter
}

type SubjectAccessReviewClientGetter interface {
	SubjectAccessReviewForCluster(request *http.Request) (v1.SubjectAccessReviewInterface, error)
}

func NewSubjectAccessReview(getter SubjectAccessReviewClientGetter) SubjectAccessReview {
	return subjectAccessReview{
		sarClientGetter: getter,
	}
}

// UserCanImpersonateUser checks if user can impersonate as impUser
func (sar subjectAccessReview) UserCanImpersonateUser(req *http.Request, user, impUser string) (bool, error) {
	userContext, err := sar.sarClientGetter.SubjectAccessReviewForCluster(req)
	if err != nil {
		return false, err
	}
	return sar.checkUserCanImpersonateUser(req.Context(), userContext, user, impUser)
}

// UserCanImpersonateGroup checks if user can impersonate as the group
func (sar subjectAccessReview) UserCanImpersonateGroup(req *http.Request, user string, group string) (bool, error) {
	userContext, err := sar.sarClientGetter.SubjectAccessReviewForCluster(req)
	if err != nil {
		return false, err
	}
	return sar.checkUserCanImpersonateGroup(req.Context(), userContext, user, group)
}

// UserCanImpersonateExtras checks if user can impersonate extras
func (sar subjectAccessReview) UserCanImpersonateExtras(req *http.Request, user string, impExtras map[string][]string) (bool, error) {
	userContext, err := sar.sarClientGetter.SubjectAccessReviewForCluster(req)
	if err != nil {
		return false, err
	}
	return sar.checkUserCanImpersonateExtras(req.Context(), userContext, user, impExtras)
}

// UserCanImpersonateServiceAccount checks if user can impersonate as the service account
func (sar subjectAccessReview) UserCanImpersonateServiceAccount(req *http.Request, user string, sa string) (bool, error) {
	userContext, err := sar.sarClientGetter.SubjectAccessReviewForCluster(req)
	if err != nil {
		return false, err
	}
	return sar.checkUserCanImpersonateServiceAccount(req.Context(), userContext, user, sa)
}

func (sar subjectAccessReview) checkUserCanImpersonateUser(ctx context.Context, sarClient v1.SubjectAccessReviewInterface, user, impUser string) (bool, error) {
	review := authV1.SubjectAccessReview{
		Spec: authV1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authV1.ResourceAttributes{
				Verb:     "impersonate",
				Resource: "users",
				Name:     impUser,
			},
		},
	}

	result, err := sarClient.Create(ctx, &review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	logrus.Debugf("Impersonate check result: %v", result)
	return result.Status.Allowed, nil
}

func (sar subjectAccessReview) checkUserCanImpersonateGroup(ctx context.Context, sarClient v1.SubjectAccessReviewInterface, user string, group string) (bool, error) {
	review := authV1.SubjectAccessReview{
		Spec: authV1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authV1.ResourceAttributes{
				Verb:     "impersonate",
				Resource: "groups",
				Name:     group,
			},
		},
	}

	result, err := sarClient.Create(ctx, &review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	logrus.Debugf("Impersonate check result: %v", result)

	return result.Status.Allowed, nil
}

func (sar subjectAccessReview) checkUserCanImpersonateExtras(ctx context.Context, sarClient v1.SubjectAccessReviewInterface, user string, extras map[string][]string) (bool, error) {
	for name, values := range extras {
		for _, value := range values {
			review := authV1.SubjectAccessReview{
				Spec: authV1.SubjectAccessReviewSpec{
					User: user,
					ResourceAttributes: &authV1.ResourceAttributes{
						Verb:     "impersonate",
						Resource: "userextras/" + name,
						Name:     value,
					},
				},
			}

			result, err := sarClient.Create(ctx, &review, metav1.CreateOptions{})
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

func (sar subjectAccessReview) checkUserCanImpersonateServiceAccount(ctx context.Context, sarClient v1.SubjectAccessReviewInterface, user string, sa string) (bool, error) {
	review := authV1.SubjectAccessReview{
		Spec: authV1.SubjectAccessReviewSpec{
			User: user,
			ResourceAttributes: &authV1.ResourceAttributes{
				Verb:     "impersonate",
				Resource: "serviceaccounts",
				Name:     sa,
			},
		},
	}

	result, err := sarClient.Create(ctx, &review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	logrus.Debugf("Impersonate sa check result: %v", result)
	return result.Status.Allowed, nil
}
