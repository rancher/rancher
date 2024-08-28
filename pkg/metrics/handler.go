package metrics

import (
	"net/http"

	"github.com/rancher/rancher/pkg/auth/util"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	authorizationv1clients "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type metricsHandler struct {
	subjectAccessReviewClient authorizationv1clients.SubjectAccessReviewInterface
	promHandler               http.Handler
}

// NewMetricsHandler ensures the user is allowed to read the Prometheus metrics endpoint
func NewMetricsHandler(scaledContextClient kubernetes.Interface, promHandler http.Handler) http.Handler {
	return &metricsHandler{
		subjectAccessReviewClient: scaledContextClient.AuthorizationV1().SubjectAccessReviews(),
		promHandler:               promHandler,
	}
}

func (h *metricsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	review := authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   req.Header.Get("Impersonate-User"),
			Groups: req.Header["Impersonate-Groups"],
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     "get",
				Resource: "ranchermetrics",
				Group:    "management.cattle.io",
			},
		},
	}

	result, err := h.subjectAccessReviewClient.Create(req.Context(), &review, metav1.CreateOptions{})
	if err != nil {
		util.ReturnHTTPError(rw, req, 500, err.Error())
		return
	}

	if !result.Status.Allowed {
		util.ReturnHTTPError(rw, req, 401, "Unauthorized")
		return
	}

	h.promHandler.ServeHTTP(rw, req)
}
