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
	// UserCanImpersonateGroups checks if user can impersonate as the groups
	UserCanImpersonateGroups(req *http.Request, user string, groups []string) (bool, error)
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

func (sar subjectAccessReview) UserCanImpersonateUser(req *http.Request, user, impUser string) (bool, error) {
	userContext, err := sar.sarClientGetter.SubjectAccessReviewForCluster(req)
	if err != nil {
		return false, err
	}
	return sar.checkUserCanImpersonateUser(userContext, user, impUser)
}

func (sar subjectAccessReview) UserCanImpersonateGroups(req *http.Request, user string, groups []string) (bool, error) {
	userContext, err := sar.sarClientGetter.SubjectAccessReviewForCluster(req)
	if err != nil {
		return false, err
	}
	return sar.checkUserCanImpersonateGroup(userContext, user, groups)
}

func (sar subjectAccessReview) checkUserCanImpersonateUser(sarClient v1.SubjectAccessReviewInterface, user, impUser string) (bool, error) {
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

	result, err := sarClient.Create(context.TODO(), &review, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	logrus.Debugf("Impersonate check result: %v", result)
	return result.Status.Allowed, nil
}

func (sar subjectAccessReview) checkUserCanImpersonateGroup(sarClient v1.SubjectAccessReviewInterface, user string, groups []string) (bool, error) {
	for _, group := range groups {
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

		result, err := sarClient.Create(context.TODO(), &review, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
		logrus.Debugf("Impersonate check result: %v", result)
		if !result.Status.Allowed {
			return false, nil
		}
	}

	return true, nil
}
