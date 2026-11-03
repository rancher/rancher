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
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
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
			req = req.WithContext(request.WithUser(req.Context(), &user.DefaultInfo{
				Name:   "user-1",
				Groups: []string{"group-1", "group-2"},
			}))
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, test.expectedStatus, rr.Code)
			assert.Equal(t, test.expectNext, nextCalled)
		})
	}
}

func TestNewSubjectAccessReviewHandlerUntrustedImpersonateHeaders(t *testing.T) {
	tests := map[string]struct {
		forgedUser   string
		forgedGroups []string
	}{
		"forged cluster-admin user header": {
			forgedUser:   "cluster-admin",
			forgedGroups: nil,
		},
		"forged system:masters group header": {
			forgedUser:   "nobody",
			forgedGroups: []string{"system:masters"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resourceAttributes := &authV1.ResourceAttributes{
				Verb:     "get",
				Resource: "secrets",
			}

			// The SAR backend would return Allowed: true if ever called — but after
			// the fix it must never be reached for a request with no auth context.
			var sarCalled bool
			client := stubSubjectAccessReviewClient{
				createFunc: func(_ context.Context, _ *authV1.SubjectAccessReview, _ metav1.CreateOptions) (*authV1.SubjectAccessReview, error) {
					sarCalled = true
					return &authV1.SubjectAccessReview{
						Status: authV1.SubjectAccessReviewStatus{Allowed: true},
					}, nil
				},
			}

			var nextCalled bool
			handler := NewSubjectAccessReviewHandler(client, resourceAttributes)(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				nextCalled = true
				rw.WriteHeader(http.StatusOK)
			}))

			// The request carries no real authentication — only forged headers and no
			// authenticated user in the context.
			req := httptest.NewRequest(http.MethodGet, "/v1/secrets", nil)
			req.Header.Set("Impersonate-User", test.forgedUser)
			for _, g := range test.forgedGroups {
				req.Header.Add("Impersonate-Group", g)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// The SAR must not be called — the middleware should reject the request
			// before reaching the authorisation check.
			assert.False(t, sarCalled,
				"SAR must not be called when there is no authenticated user in the request context (case %q)", name)
			assert.False(t, nextCalled,
				"handler must not admit a request with no authenticated user in context (case %q)", name)
			assert.Equal(t, http.StatusUnauthorized, rr.Code,
				"handler must return 401 when there is no authenticated user in context (case %q)", name)
		})
	}
}

type stubSubjectAccessReviewClient struct {
	createFunc func(ctx context.Context, review *authV1.SubjectAccessReview, opts metav1.CreateOptions) (*authV1.SubjectAccessReview, error)
}

func (s stubSubjectAccessReviewClient) Create(ctx context.Context, review *authV1.SubjectAccessReview, opts metav1.CreateOptions) (*authV1.SubjectAccessReview, error) {
	return s.createFunc(ctx, review, opts)
}
