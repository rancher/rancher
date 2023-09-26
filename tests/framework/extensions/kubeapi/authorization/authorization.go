package authorization

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// WaitForAllowed verifies access to resources using the SelfSubjectAccessReview
// API. It returns nil when all have been granted within some defined timeout,
// otherwise it returns an error.
func WaitForAllowed(client *rancher.Client, clusterID string, attrs []*authzv1.ResourceAttributes) error {
	// 40 seconds ought to do it
	backoff := kwait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    81,
	}
	err := kwait.ExponentialBackoff(backoff, func() (done bool, err error) {
		for _, attr := range attrs {
			selfReview := &authzv1.SelfSubjectAccessReview{
				Spec: authzv1.SelfSubjectAccessReviewSpec{
					ResourceAttributes: attr,
				},
			}

			selfSARResource, err := kubeapi.ResourceForClient(client, clusterID, "", schema.GroupVersionResource{
				Group:    "authorization.k8s.io",
				Version:  "v1",
				Resource: "selfsubjectaccessreviews",
			})
			if err != nil {
				return false, err
			}

			respUnstructured, err := selfSARResource.Create(context.TODO(), unstructured.MustToUnstructured(selfReview), metav1.CreateOptions{})
			if err != nil {
				return false, nil
			}

			selfReviewResp := &authzv1.SelfSubjectAccessReview{}
			err = scheme.Scheme.Convert(respUnstructured, selfReviewResp, respUnstructured.GroupVersionKind())
			if err != nil {
				return false, err
			}

			if !selfReviewResp.Status.Allowed {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("not all access were granted: %w", err)
	}
	return nil
}
