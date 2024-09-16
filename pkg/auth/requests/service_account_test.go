package requests

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	jwtv4 "github.com/golang-jwt/jwt/v4"
	"github.com/gorilla/mux"
	v3api "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/types/config"
	wranglerfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	k8sRequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

const (
	proxyPrefix       = "/k8s/clusters/"
	tokenReviewSuffix = "/apis/authentication.k8s.io/v1/tokenreviews"
)

func TestIsTokenExpired(t *testing.T) {
	tests := []struct {
		name           string
		expirationTime *jwtv4.NumericDate
		wantExpired    bool
	}{
		{
			name:        "empty expiration",
			wantExpired: false,
		},
		{
			name:           "expired token",
			expirationTime: jwtv4.NewNumericDate(time.Now().Add(-time.Hour)),
			wantExpired:    true,
		},
		{
			name:           "not expired token",
			expirationTime: jwtv4.NewNumericDate(time.Now().Add(time.Hour)),
			wantExpired:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testClaim := jwtv4.RegisteredClaims{
				ExpiresAt: tt.expirationTime,
			}
			if got := isTokenExpired(testClaim); got != tt.wantExpired {
				t.Errorf("isTokenExpired() = %v, wantExpired %v", got, tt.wantExpired)
			}
		})
	}
}

func TestServiceAccountAuthAuthenticate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                  string
		startUser             string
		expectedUser          string
		settingEnabled        bool
		token                 string
		tokenReviewAuthStatus bool
		downstreamRequest     *http.Request
		clusterID             string
		isAuthenticated       bool
		expectedError         bool
		omitProxyConfig       bool
	}{
		{
			name: "everything valid auth succeeds secure mode",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:             "system:cattle:error",
			expectedUser:          "expected-username",
			settingEnabled:        true,
			tokenReviewAuthStatus: true,
			downstreamRequest:     generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
			clusterID:             "testclusterid",
			isAuthenticated:       true,
			expectedError:         false,
		},
		{
			name: "everything valid auth succeeds insecure mode",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:             "system:cattle:error",
			expectedUser:          "expected-username",
			settingEnabled:        true,
			tokenReviewAuthStatus: true,
			downstreamRequest:     generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
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
			settingEnabled:        true,
			tokenReviewAuthStatus: false,
			downstreamRequest:     generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
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
			startUser:         "system:cattle:error",
			expectedUser:      "system:cattle:error",
			settingEnabled:    true,
			downstreamRequest: generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
			clusterID:         "testclusterid",
			isAuthenticated:   false,
			expectedError:     false,
		},
		{
			name:              "invalid token auth fail",
			token:             "totallybogusjwttoken",
			startUser:         "system:cattle:error",
			expectedUser:      "system:cattle:error",
			settingEnabled:    true,
			downstreamRequest: generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
			clusterID:         "testclusterid",
			isAuthenticated:   false,
			expectedError:     true,
		},
		{
			name: "invalid subject in jwt will fail",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "not-a-sa:totalfail",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:         "system:cattle:error",
			expectedUser:      "system:cattle:error",
			settingEnabled:    true,
			downstreamRequest: generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
			clusterID:         "testcluster",
			isAuthenticated:   false,
			expectedError:     false,
		},
		{
			name: "missing clusterid auth fail",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:         "system:cattle:error",
			expectedUser:      "system:cattle:error",
			settingEnabled:    true,
			downstreamRequest: generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
			clusterID:         "",
			isAuthenticated:   true,
			expectedError:     true,
		},
		{
			name: "everything valid but setting is off auth fails",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:         "system:cattle:error",
			expectedUser:      "system:cattle:error",
			settingEnabled:    false,
			downstreamRequest: generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
			clusterID:         "testcluster",
			isAuthenticated:   false,
			expectedError:     false,
		},
		{
			name: "everything valid but setting is missing auth fails",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:         "system:cattle:error",
			expectedUser:      "system:cattle:error",
			downstreamRequest: generateDownstreamRequest("POST", "testclusterid"+tokenReviewSuffix),
			clusterID:         "testcluster",
			isAuthenticated:   false,
			expectedError:     false,
			omitProxyConfig:   true,
		},
		{
			name: "everything valid auth succeeds for non tokenreview feature enabled",
			token: makeJWT(jwtv4.MapClaims{
				"sub": "system:serviceaccount:vault:token-reviewer",
				"exp": time.Now().Add(time.Hour).Unix(),
			}),
			startUser:             "system:cattle:error",
			expectedUser:          "expected-username",
			settingEnabled:        true,
			tokenReviewAuthStatus: true,
			downstreamRequest:     generateDownstreamRequest("GET", "testclusterid/apis/testgroup.k8s.io/v1/testendpoint"),
			clusterID:             "testclusterid",
			isAuthenticated:       true,
			expectedError:         false,
		},
	}

	for _, test := range tests {
		test := test
		ctrl := gomock.NewController(t)
		cpsCache := wranglerfake.NewMockCacheInterface[*v3api.ClusterProxyConfig](ctrl)
		cpsCache.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(namespace string, _ labels.Selector) ([]*v3api.ClusterProxyConfig, error) {
			if test.omitProxyConfig {
				return nil, nil
			}
			return []*v3api.ClusterProxyConfig{
				{
					Enabled: test.settingEnabled,
				},
			}, nil
		}).AnyTimes()
		t.Run(test.name, func(t *testing.T) {
			mgmtCtx, err := config.NewScaledContext(rest.Config{}, nil)
			auth := &ServiceAccountAuth{
				scaledContext: mgmtCtx,
				clusterLister: &fakes.ClusterListerMock{
					GetFunc: func(namespace string, name string) (*v3.Cluster, error) {
						return &v3.Cluster{}, nil
					},
				},
				secretLister: nil,
				restConfigGetter: func(cluster *v3.Cluster, context *config.ScaledContext, secretLister corev1.SecretLister, tryReconnecting bool) (*rest.Config, error) {
					return &rest.Config{}, nil
				},
				authClientCreator: func(clusterID string) (kubernetes.Interface, error) {
					authclient := fake.NewSimpleClientset()
					// Use PrependReactor because there is a default Reactor for */* that will trump ours if we just use AddReactor
					authclient.Fake.PrependReactor("create", "tokenreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
						tokenReview := &v1.TokenReview{
							TypeMeta: metav1.TypeMeta{
								Kind:       "TokenReview",
								APIVersion: "authentication.k8s.io/v1",
							},
							Status: v1.TokenReviewStatus{
								Authenticated: test.tokenReviewAuthStatus,
								Audiences:     []string{},
								User: v1.UserInfo{
									Username: test.expectedUser,
								},
							},
						}
						return true, tokenReview, nil
					})
					return authclient, nil
				},
				clusterProxyConfigsGetter: cpsCache,
			}

			req := test.downstreamRequest
			req.Header.Set("Authorization", "Bearer "+test.token)
			startUserInfo := &user.DefaultInfo{
				Name: test.startUser,
			}
			currentContext := req.Context()
			req = req.WithContext(k8sRequest.WithUser(currentContext, startUserInfo))
			req = mux.SetURLVars(req, map[string]string{"clusterID": test.clusterID})
			user, isAuthenticated, err := auth.Authenticate(req.WithContext(req.Context()))

			if test.expectedError {
				assert.NotNil(t, err, "Expected error")
			} else {
				assert.Nil(t, err, "Expected no error")
			}
			assert.Equal(t, test.isAuthenticated, isAuthenticated, "Unexpected authentication result")
			assert.Equal(t, test.expectedUser, user.GetName(), "Expected username in the user info")
		})
	}
}

func generateDownstreamRequest(method string, path string) *http.Request {
	req, _ := http.NewRequest(method, proxyPrefix+path, nil)
	return req
}

func makeJWT(claims jwtv4.Claims) string {
	token := jwtv4.NewWithClaims(jwtv4.SigningMethodHS256, claims)
	sampleSecretKey := []byte("testingkeyfortestpurposes")
	tokenString, _ := token.SignedString(sampleSecretKey)
	return tokenString
}

func TestConvertExtra(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]v1.ExtraValue
		want  map[string][]string
	}{
		{
			name:  "Empty Input",
			input: map[string]v1.ExtraValue{},
			want:  map[string][]string{},
		},
		{
			name: "Single Key-Value",
			input: map[string]v1.ExtraValue{
				"key1": {"value1"},
			},
			want: map[string][]string{
				"key1": {"value1"},
			},
		},
		{
			name: "Multiple Key-Values",
			input: map[string]v1.ExtraValue{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
			want: map[string][]string{
				"key1": {"value1", "value2"},
				"key2": {"value3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertExtra(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertExtra() = %v, wantExpired %v", got, tt.want)
			}
		})
	}
}
