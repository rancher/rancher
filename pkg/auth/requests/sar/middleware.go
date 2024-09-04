package sar

import (
	"net/http"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"

	"github.com/rancher/rancher/pkg/auth/util"
)

// NewSubjectAccessReviewHandler provides an HTTP middleware validating that the user has access to the provided resource attributes.
// User and groups information is taken from Impersonate-User and Impersonate-Group headers from the HTTP request
func NewSubjectAccessReviewHandler(client authorizationv1client.SubjectAccessReviewInterface, resourceAttributes *authv1.ResourceAttributes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			user := req.Header.Get("Impersonate-User")
			groups := req.Header["Impersonate-Groups"]
			review := authv1.SubjectAccessReview{
				Spec: authv1.SubjectAccessReviewSpec{
					User:               user,
					Groups:             groups,
					ResourceAttributes: resourceAttributes,
				},
			}

			result, err := client.Create(req.Context(), &review, metav1.CreateOptions{})
			if err != nil {
				util.ReturnHTTPError(rw, req, http.StatusInternalServerError, err.Error())
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
