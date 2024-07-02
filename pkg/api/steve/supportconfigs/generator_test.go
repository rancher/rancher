package supportconfigs

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	configmapfakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/rancher/pkg/managedcharts/cspadapter"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/release"
	authv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type FakeChartUtil struct {
	generateError    bool // return a random error
	generateNotFound bool // return cspadapter.ErrNotFound
}

func NewFakeChartUtil(generateError bool, generateNotFound bool) *FakeChartUtil {
	return &FakeChartUtil{
		generateError:    generateError,
		generateNotFound: generateNotFound,
	}
}

func (c *FakeChartUtil) GetRelease(_ string, _ string) (*release.Release, error) {
	if c.generateError {
		return nil, fmt.Errorf("random error")
	}
	// indiscriminately generate a not found error
	if c.generateNotFound {
		return nil, cspadapter.ErrNotFound
	}
	// NOTE: we don't actually have to return a helm release as it will be
	// ignored by the caller
	return &release.Release{}, nil
}

func TestGenerateSupportConfigScenarios(t *testing.T) {
	scenarios := []struct {
		name                            string
		usePAYG                         bool
		generateAdapterError            bool
		generateAdapterNotFound         bool
		generateAuthorizedError         bool
		generateMeteringArchiveNotFound bool
		authorized                      bool
		marshalledCSPConfig             string
		expectedHTTPCode                int
	}{
		{
			name:                    "internal server error due to CSP release lookup",
			usePAYG:                 true,
			generateAdapterError:    true,
			generateAdapterNotFound: false,
			generateAuthorizedError: false,
			authorized:              true,
			expectedHTTPCode:        http.StatusInternalServerError,
		},
		{
			name:                    "denied access due to error while performing authorization",
			usePAYG:                 true,
			generateAdapterError:    false,
			generateAdapterNotFound: false,
			generateAuthorizedError: true,
			authorized:              false,
			marshalledCSPConfig:     "{}",
			expectedHTTPCode:        http.StatusForbidden,
		},
		{
			name:                    "user requests PAYG, authorized to get PAYG, PAYG not installed (501)",
			usePAYG:                 true,
			generateAdapterError:    false,
			generateAdapterNotFound: true,
			generateAuthorizedError: false,
			authorized:              true,
			marshalledCSPConfig:     "{}",
			expectedHTTPCode:        http.StatusNotImplemented,
		},
		{
			name:                    "user requests PAYG, not authorized to get PAYG, auth denied (403)",
			usePAYG:                 true,
			generateAdapterError:    false,
			generateAdapterNotFound: true,
			generateAuthorizedError: false,
			authorized:              false,
			marshalledCSPConfig:     "{}",
			expectedHTTPCode:        http.StatusForbidden,
		},
		{
			name:                    "user requests BYOL, authorized to get BYOL, BYOL not installed (501)",
			usePAYG:                 false,
			generateAdapterError:    false,
			generateAdapterNotFound: true,
			generateAuthorizedError: false,
			authorized:              true,
			marshalledCSPConfig:     "{}",
			expectedHTTPCode:        http.StatusNotImplemented,
		},
		{
			name:                    "user requests BYOL, not authorized to get BYOL, auth denied (403)",
			usePAYG:                 false,
			generateAdapterError:    false,
			generateAdapterNotFound: false,
			generateAuthorizedError: false,
			authorized:              false,
			marshalledCSPConfig:     "{}",
			expectedHTTPCode:        http.StatusForbidden,
		},
		{
			name:                    "user requests BYOL, authorized to get BYOL, we return output (200)",
			usePAYG:                 false,
			generateAdapterError:    false,
			generateAdapterNotFound: false,
			generateAuthorizedError: false,
			authorized:              true,
			marshalledCSPConfig:     "{}",
			expectedHTTPCode:        http.StatusOK,
		},
		{
			name:                    "user requests PAYG, autorized to get PAYG, we return output (200)",
			usePAYG:                 true,
			generateAdapterError:    false,
			generateAdapterNotFound: false,
			generateAuthorizedError: false,
			authorized:              true,
			marshalledCSPConfig:     "{}",
			expectedHTTPCode:        http.StatusOK,
		},
		{
			name:                            "user requests PAYG, authorized to get PAYG, metering-archive is not available, we return output (200) ",
			usePAYG:                         true,
			generateAdapterError:            false,
			generateAdapterNotFound:         false,
			generateAuthorizedError:         false,
			generateMeteringArchiveNotFound: true,
			authorized:                      true,
			marshalledCSPConfig:             "{}",
			expectedHTTPCode:                http.StatusOK,
		},
	}
	for _, scenario := range scenarios {
		test := scenario
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var k8sClient *fake.Clientset
			objs := []runtime.Object{}
			k8sClient = fake.NewSimpleClientset(objs...)
			k8sClient.PrependReactor("create", "subjectaccessreviews",
				func(action k8stesting.Action) (bool, runtime.Object, error) {
					ret := action.(k8stesting.CreateAction).GetObject().(*authv1.SubjectAccessReview)
					if test.generateAuthorizedError {
						return false, nil, fmt.Errorf("random error")
					}
					ret.Status.Allowed = test.authorized
					return true, ret, nil
				},
			)
			h := &Handler{
				ConfigMaps: &configmapfakes.ConfigMapInterfaceMock{
					GetNamespacedFunc: func(namespace string, name string, opts metav1.GetOptions) (*v1.ConfigMap, error) {
						// NOTE: we are not testing the configmap itself. Just need to return a valid configmap.
						if name == cspAdapterConfigmap {
							return &v1.ConfigMap{
								Data: map[string]string{
									"data": "{}",
								},
							}, nil
						} else {
							if test.generateMeteringArchiveNotFound {
								return nil, errNotFound
							}
							return &v1.ConfigMap{
								Data: map[string]string{
									"archive": "[]",
								},
							}, nil
						}
					},
				},
				SubjectAccessReviews: k8sClient.AuthorizationV1().SubjectAccessReviews(),
				adapterUtil:          NewFakeChartUtil(test.generateAdapterError, test.generateAdapterNotFound),
			}
			reqPath := "/v1/generateSUSERancherSupportConfig"
			if test.usePAYG {
				reqPath = reqPath + "?usePAYG=true"
			}
			req := httptest.NewRequest(http.MethodGet, reqPath, nil)
			rr := httptest.NewRecorder()
			ctx := req.Context()
			ctx = request.WithUser(
				ctx,
				&user.DefaultInfo{
					Name:   "foo",
					UID:    "18",
					Groups: []string{"foogroup"},
					Extra:  map[string][]string{"foo": {"bar"}},
				},
			)
			req = req.WithContext(ctx)
			h.ServeHTTP(rr, req)
			assert.Equal(t, test.expectedHTTPCode, rr.Code)
			if test.expectedHTTPCode == http.StatusForbidden {
				// if user denied access, config.json should not be returned
				body, _ := io.ReadAll(rr.Body)
				assert.False(t, strings.Contains(string(body), "rancher/config.json"))
			}
			if test.generateMeteringArchiveNotFound {
				body, _ := io.ReadAll(rr.Body)
				assert.False(t, strings.Contains(string(body), "rancher/metering_archive.json"))
			}
		})
	}
}
