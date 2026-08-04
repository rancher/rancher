package requests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	k8srequest "k8s.io/apiserver/pkg/endpoints/request"
	authv1client "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"
)

func TestTokenReviewAuthAuthenticate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		userInfo         user.Info
		authorization    string
		reviewResponse   *authenticationv1.TokenReview
		reviewErr        string
		wantName         string
		wantUID          string
		wantGroups       []string
		wantHasAuth      bool
		wantReviewCalled bool
		wantReviewToken  string
	}{
		"already authenticated user bypasses token review": {
			userInfo: &user.DefaultInfo{
				Name:   "u-123",
				UID:    "u-123",
				Groups: []string{"devs"},
			},
			wantName:    "u-123",
			wantUID:     "u-123",
			wantGroups:  []string{"devs"},
			wantHasAuth: true,
		},
		"when the authenticated user is the error user a token review is done": {
			userInfo: &user.DefaultInfo{
				Name: errorUserName,
			},
			authorization:    "Bearer token-review-token",
			wantReviewCalled: true,
			wantReviewToken:  "token-review-token",
			reviewResponse: &authenticationv1.TokenReview{
				Status: authenticationv1.TokenReviewStatus{
					Authenticated: false,
					User: authenticationv1.UserInfo{
						Username: "u-123",
						UID:      "u-123",
						Groups:   []string{"devs"},
					},
				},
			},
			wantName:    "u-123",
			wantUID:     "u-123",
			wantGroups:  []string{"devs"},
			wantHasAuth: false,
		},
		"failed auth and token review return error": {
			userInfo: &user.DefaultInfo{
				Name: "system:cattle:error",
			},
			authorization:    "Bearer token-review-token",
			reviewErr:        "token review create failed",
			wantName:         "system:cattle:error",
			wantHasAuth:      false,
			wantReviewCalled: true,
			wantReviewToken:  "token-review-token",
		},
		"failed auth and token review success returns review user as authenticated": {
			userInfo: &user.DefaultInfo{
				Name: "system:cattle:error",
			},
			authorization: "Bearer review-success-token",
			reviewResponse: &authenticationv1.TokenReview{
				Status: authenticationv1.TokenReviewStatus{
					Authenticated: true,
					User: authenticationv1.UserInfo{
						Username: "system:serviceaccount:default:my-sa",
						UID:      "uid-1",
						Groups:   []string{"system:serviceaccounts", "system:authenticated"},
					},
				},
			},
			wantName:         "system:serviceaccount:default:my-sa",
			wantUID:          "uid-1",
			wantGroups:       []string{"system:serviceaccounts", "system:authenticated"},
			wantHasAuth:      true,
			wantReviewCalled: true,
			wantReviewToken:  "review-success-token",
		},
		"failed auth and token review success with unauthenticated status still returns review user": {
			userInfo: &user.DefaultInfo{
				Name: errorUserName,
			},
			authorization: "Bearer review-not-authenticated-token",
			reviewResponse: &authenticationv1.TokenReview{
				Status: authenticationv1.TokenReviewStatus{
					// The token review has returns Not Authenticated
					Authenticated: false,
					User: authenticationv1.UserInfo{
						Username: "impersonated-user",
						UID:      "uid-2",
						Groups:   []string{"group-a"},
					},
				},
			},
			wantName:         "impersonated-user",
			wantUID:          "uid-2",
			wantGroups:       []string{"group-a"},
			wantHasAuth:      false,
			wantReviewCalled: true,
			wantReviewToken:  "review-not-authenticated-token",
		},
		"token review status error with unauthenticated status should not authenticate": {
			userInfo: &user.DefaultInfo{
				Name: errorUserName,
			},
			authorization: "Bearer some-token",
			reviewResponse: &authenticationv1.TokenReview{
				Status: authenticationv1.TokenReviewStatus{
					Authenticated: false,
					Error:         "token has been revoked",
				},
			},
			wantName:         errorUserName,
			wantHasAuth:      false,
			wantReviewCalled: true,
			wantReviewToken:  "some-token",
		},
		"token review status error with authenticated true should not authenticate": {
			userInfo: &user.DefaultInfo{
				Name: errorUserName,
			},
			authorization: "Bearer some-token",
			reviewResponse: &authenticationv1.TokenReview{
				Status: authenticationv1.TokenReviewStatus{
					Authenticated: true,
					Error:         "token lookup failed",
					User: authenticationv1.UserInfo{
						Username: "some-user",
						UID:      "uid-3",
					},
				},
			},
			wantName:         errorUserName,
			wantHasAuth:      false,
			wantReviewCalled: true,
			wantReviewToken:  "some-token",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			reviewCalled := 0
			var reviewToken string

			authClient := &fakeAuthenticationV1{
				tokenReviews: &fakeTokenReviewInterface{
					create: func(_ context.Context, createdReview *authenticationv1.TokenReview, _ metav1.CreateOptions) (*authenticationv1.TokenReview, error) {
						reviewCalled++
						reviewToken = createdReview.Spec.Token

						if tt.reviewErr != "" {
							return nil, errors.New(tt.reviewErr)
						}

						if tt.reviewResponse != nil {
							return tt.reviewResponse, nil
						}

						return &authenticationv1.TokenReview{}, nil
					}},
			}

			authenticator := &TokenReviewAuth{AuthClient: authClient}

			req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			req = req.WithContext(k8srequest.WithUser(req.Context(), tt.userInfo))

			respUser, respHasAuth, err := authenticator.Authenticate(req)
			require.NoError(t, err)

			assert.Equal(t, tt.wantHasAuth, respHasAuth)
			assert.Equal(t, tt.wantName, respUser.GetName())
			assert.Equal(t, tt.wantUID, respUser.GetUID())
			assert.Equal(t, tt.wantGroups, respUser.GetGroups())

			if tt.wantReviewCalled {
				require.Equal(t, 1, reviewCalled)
				assert.Equal(t, tt.wantReviewToken, reviewToken)
			} else {
				assert.Equal(t, 0, reviewCalled)
			}
		})
	}
}

type fakeAuthenticationV1 struct {
	tokenReviews authv1client.TokenReviewInterface
}

func (f *fakeAuthenticationV1) RESTClient() rest.Interface {
	return nil
}

func (f *fakeAuthenticationV1) SelfSubjectReviews() authv1client.SelfSubjectReviewInterface {
	return nil
}

func (f *fakeAuthenticationV1) TokenReviews() authv1client.TokenReviewInterface {
	return f.tokenReviews
}

type fakeTokenReviewInterface struct {
	create func(ctx context.Context, tokenReview *authenticationv1.TokenReview, opts metav1.CreateOptions) (*authenticationv1.TokenReview, error)
}

func (f *fakeTokenReviewInterface) Create(ctx context.Context, tokenReview *authenticationv1.TokenReview, opts metav1.CreateOptions) (*authenticationv1.TokenReview, error) {
	return f.create(ctx, tokenReview, opts)
}
