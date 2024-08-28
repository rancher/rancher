package metrics

import (
	"net/http"

	"github.com/rancher/rancher/pkg/auth/util"
	v2 "k8s.io/api/authorization/v1"
	v3 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type metricsHandler struct {
	subjectAccessReviewClient v1.SubjectAccessReviewInterface
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
	review := v2.SubjectAccessReview{
		Spec: v2.SubjectAccessReviewSpec{
			User:   req.Header.Get("Impersonate-User"),
			Groups: req.Header["Impersonate-Groups"],
			ResourceAttributes: &v2.ResourceAttributes{
				Verb:     "get",
				Resource: "ranchermetrics",
				Group:    "management.cattle.io",
			},
		},
	}

	result, err := h.subjectAccessReviewClient.Create(req.Context(), &review, v3.CreateOptions{})
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
