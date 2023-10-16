package requests

import (
	"net/http"
	"testing"
	"time"

	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	k8sRequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/fake"
	authv1 "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestIsTokenExpired(t *testing.T) {
	tests := []struct {
		name           string
		expirationTime *jwtv4.NumericDate
		want           bool
	}{
		{
			name: "empty expiration",
			want: false,
		},
		{
			name:           "expired token",
			expirationTime: jwtv4.NewNumericDate(time.Now().Add(-time.Hour)),
			want:           true,
		},
		{
			name:           "not expired token",
			expirationTime: jwtv4.NewNumericDate(time.Now().Add(time.Hour)),
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTokenExpired(tt.expirationTime); got != tt.want {
				t.Errorf("isTokenExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	tests := []struct {
		name                  string
		startUser             string
		expectedUser          string
		token                 string
		tokenReviewAuthStatus bool
		clusterID             string
		isAuthenticated       bool
		expectedError         bool
	}{
		{
			name: "everything valid auth succeeds",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:             "system:cattle:error",
			expectedUser:          "expected-username",
			tokenReviewAuthStatus: true,
			clusterID:             "testclusterid",
			isAuthenticated:       true,
			expectedError:         false,
		},
		{
			name: "everything valid but tokenreview auth fails",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:             "system:cattle:error",
			expectedUser:          "",
			tokenReviewAuthStatus: false,
			clusterID:             "testclusterid",
			isAuthenticated:       false,
			expectedError:         false,
		},
		{
			name: "expired token auth fails",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(-time.Hour).Unix(),
			}),
			startUser:       "system:cattle:error",
			expectedUser:    "system:cattle:error",
			clusterID:       "testclusterid",
			isAuthenticated: false,
			expectedError:   false,
		},
		{
			name:            "invalid token auth fail",
			token:           "totallybogusjwttoken",
			startUser:       "system:cattle:error",
			expectedUser:    "system:cattle:error",
			clusterID:       "testclusterid",
			isAuthenticated: false,
			expectedError:   true,
		},
		{
			name: "invalid subject in jwt will fail",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "not-a-sa:totalfail",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:       "system:cattle:error",
			expectedUser:    "system:cattle:error",
			clusterID:       "testcluster",
			isAuthenticated: false,
			expectedError:   false,
		},
		{
			name: "missing clusterid auth fail",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:       "system:cattle:error",
			expectedUser:    "system:cattle:error",
			clusterID:       "",
			isAuthenticated: true,
			expectedError:   true,
		},
		{
			startUser:       "anotherauthuser",
			expectedUser:    "anotherauthuser",
			isAuthenticated: true,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgmtCtx, err := config.NewScaledContext(rest.Config{}, nil)
			auth := &ServiceAccountAuth{
				scaledContext: mgmtCtx,
				clusterLister: &fakes.ClusterListerMock{
					GetFunc: func(namespace string, name string) (*v3.Cluster, error) {
						return &v3.Cluster{}, nil
					},
				},
				secretLister: nil,
				restConfigGetter: func(cluster *v3.Cluster, context *config.ScaledContext, secretLister corev1.SecretLister) (*rest.Config, error) {
					return &rest.Config{}, nil
				},
				authClientCreator: func(config *rest.Config) (authv1.AuthenticationV1Interface, error) {
					authclient := fake.NewSimpleClientset()
					// Use PrependReactor because there is a default Reactor for */* that will trump ours if we just use AddReactor
					authclient.Fake.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
						tokenReview := &v1.TokenReview{
							TypeMeta: metav1.TypeMeta{
								Kind:       "TokenReview",
								APIVersion: "authentication.k8s.io/v1",
							},
							Status: v1.TokenReviewStatus{
								Authenticated: tt.tokenReviewAuthStatus,
								Audiences:     []string{},
								User: v1.UserInfo{
									Username: tt.expectedUser,
								},
							},
						}
						return true, tokenReview, nil
					})
					return authclient.AuthenticationV1(), nil
				},
			}

			req, _ := http.NewRequest("GET", "/k8s/clusters/"+tt.clusterID+"/apis/authentication.k8s.io/tokenreviews", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			startUserInfo := &user.DefaultInfo{
				Name: tt.startUser,
			}
			currentContext := req.Context()
			req = req.WithContext(k8sRequest.WithUser(currentContext, startUserInfo))
			req = mux.SetURLVars(req, map[string]string{"clusterID": tt.clusterID})
			user, isAuthenticated, err := auth.Authenticate(req.WithContext(req.Context()))

			if tt.expectedError {
				assert.NotNil(t, err, "Expected error")
			} else {
				assert.Nil(t, err, "Expected no error")
			}
			assert.Equal(t, tt.isAuthenticated, isAuthenticated, "Unexpected authentication result")
			assert.Equal(t, tt.expectedUser, user.GetName(), "Expected username in the user info")
		})
	}
}

func makeJWT(claims jwtv4.Claims) string {
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodHS256, claims)
	sampleSecretKey := []byte("testingkeyfortestpurposes")
	tokenString, _ := token.SignedString(sampleSecretKey)
	return tokenString
}
