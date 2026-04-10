package sar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	authV1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewSubjectAccessReviewHandler(t *testing.T) {
	tests := map[string]struct {
		result         *authV1.SubjectAccessReview
		createErr      error
		expectedStatus int
		expectNext     bool
		assertReview   func(t *testing.T, review *authV1.SubjectAccessReview)
	}{
		"allowed request": {
			result: &authV1.SubjectAccessReview{
				Status: authV1.SubjectAccessReviewStatus{Allowed: true},
			},
			expectedStatus: http.StatusTeapot,
			expectNext:     true,
			assertReview: func(t *testing.T, review *authV1.SubjectAccessReview) {
				assert.Equal(t, "user-1", review.Spec.User)
				assert.Equal(t, []string{"group-1", "group-2"}, review.Spec.Groups)
				if assert.NotNil(t, review.Spec.ResourceAttributes) {
					assert.Equal(t, "get", review.Spec.ResourceAttributes.Verb)
					assert.Equal(t, "clusters", review.Spec.ResourceAttributes.Resource)
					assert.Equal(t, "c-1", review.Spec.ResourceAttributes.Name)
				}
			},
		},
		"denied request": {
			result: &authV1.SubjectAccessReview{
				Status: authV1.SubjectAccessReviewStatus{Allowed: false},
			},
			expectedStatus: http.StatusUnauthorized,
			expectNext:     false,
			assertReview: func(t *testing.T, review *authV1.SubjectAccessReview) {
				assert.Equal(t, "user-1", review.Spec.User)
				assert.Equal(t, []string{"group-1", "group-2"}, review.Spec.Groups)
			},
		},
		"create error": {
			createErr:      errors.New("backend unavailable"),
			expectedStatus: http.StatusInternalServerError,
			expectNext:     false,
			assertReview: func(t *testing.T, review *authV1.SubjectAccessReview) {
				assert.Equal(t, "user-1", review.Spec.User)
				assert.Equal(t, []string{"group-1", "group-2"}, review.Spec.Groups)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resourceAttributes := &authV1.ResourceAttributes{
				Verb:     "get",
				Resource: "clusters",
				Name:     "c-1",
			}

			var nextCalled bool
			client := stubSubjectAccessReviewClient{
				createFunc: func(ctx context.Context, review *authV1.SubjectAccessReview, opts metav1.CreateOptions) (*authV1.SubjectAccessReview, error) {
					assert.Equal(t, metav1.CreateOptions{}, opts)
					test.assertReview(t, review)
					return test.result, test.createErr
				},
			}

			handler := NewSubjectAccessReviewHandler(client, resourceAttributes)(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				nextCalled = true
				rw.WriteHeader(http.StatusTeapot)
			}))

			req := httptest.NewRequest(http.MethodGet, "/v1/clusters/c-1", nil)
			req.Header.Set("Impersonate-User", "user-1")
			req.Header.Add("Impersonate-Group", "group-1")
			req.Header.Add("Impersonate-Group", "group-2")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, test.expectedStatus, rr.Code)
			assert.Equal(t, test.expectNext, nextCalled)
		})
	}
}

type stubSubjectAccessReviewClient struct {
	createFunc func(ctx context.Context, review *authV1.SubjectAccessReview, opts metav1.CreateOptions) (*authV1.SubjectAccessReview, error)
}

func (s stubSubjectAccessReviewClient) Create(ctx context.Context, review *authV1.SubjectAccessReview, opts metav1.CreateOptions) (*authV1.SubjectAccessReview, error) {
	return s.createFunc(ctx, review, opts)
}
