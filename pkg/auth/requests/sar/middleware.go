package sar

import (
	"net/http"

	"github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/endpoints/request"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"

	"github.com/rancher/rancher/pkg/auth/util"
)

// NewSubjectAccessReviewHandler provides an HTTP middleware validating that the user has access to the provided resource attributes.
// User and groups information is taken from the authenticated user stored in the request context by the authentication layer.
func NewSubjectAccessReviewHandler(client authorizationv1client.SubjectAccessReviewInterface, resourceAttributes *authv1.ResourceAttributes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			userInfo, ok := request.UserFrom(req.Context())
			if !ok {
				util.ReturnHTTPError(rw, req, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
				return
			}
			review := authv1.SubjectAccessReview{
				Spec: authv1.SubjectAccessReviewSpec{
					User:               userInfo.GetName(),
					Groups:             userInfo.GetGroups(),
					ResourceAttributes: resourceAttributes,
				},
			}

			result, err := client.Create(req.Context(), &review, metav1.CreateOptions{})
			if err != nil {
				logrus.Errorf("[SAR middleware] subject access review failed: %v", err)
				util.ReturnHTTPError(rw, req, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
				return
			}

			if !result.Status.Allowed {
				util.ReturnHTTPError(rw, req, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
				return
			}

			next.ServeHTTP(rw, req)
		})
	}
}
