package cluster

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	v3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	managementSchema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/user"
	userMocks "github.com/rancher/rancher/pkg/user/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGenerateKubeconfigActionHandler(t *testing.T) {
	tests := []struct {
		name              string
		hostname          string
		generateToken     string
		clusterAceEnabled bool

		clusterLookupErr error
		nodeListerErr    error
		tokenCreateErr   error

		wantErr bool
	}{
		{
			name:          "no token generation",
			generateToken: "false",
			wantErr:       false,
		},
		{
			name:          "token generation",
			generateToken: "true",
			wantErr:       false,
		},
		{
			name:          "token generation with hostname set",
			generateToken: "true",
			hostname:      "https://set-hostname.fake",
			wantErr:       false,
		},
		{
			name:          "no token generation with hostname set",
			generateToken: "false",
			hostname:      "https://set-hostname.fake",
			wantErr:       false,
		},
	}

	const (
		testClusterName = "test-cluster"
		fakeHost        = "fake-request-host.fake"
		testUser        = ""
	)

	ctrl := gomock.NewController(t)
	userManager := userMocks.NewMockManager(ctrl)
	userManager.EXPECT().GetUser(gomock.Any()).Return(testUser).AnyTimes()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testSchemas := types.NewSchemas().AddSchemas(managementSchema.Schemas)
			clusterSchema := testSchemas.Schema(&managementSchema.Version, v3.ClusterType)
			fakeStore := fakeClusterStore{
				cluster: v3.Cluster{
					Name: testClusterName,
				},
				err: test.clusterLookupErr,
			}
			clusterSchema.Store = &fakeStore
			err := settings.KubeconfigGenerateToken.Set(test.generateToken)
			assert.NoError(t, err, "got an error when setting up kubeconfig token setting")
			err = settings.ServerURL.Set(test.hostname)
			assert.NoError(t, err, "got an error when setting up the server url setting")

			recorder := normanRecorder{}
			apiContext := &types.APIContext{
				ID:             testClusterName,
				Version:        &managementSchema.Version,
				Type:           v3.ClusterType,
				ResponseWriter: &recorder,
				Schemas:        testSchemas,
				Request:        &http.Request{Host: fakeHost},
			}

			fakeAuth := fakeAuthToken{
				token: apimgmtv3.Token{
					AuthProvider: "local",
					UserPrincipal: apimgmtv3.Principal{
						Provider: "local",
						ObjectMeta: metav1.ObjectMeta{
							Name: testUser,
						},
					},
				},
			}

			handler := ActionHandler{
				NodeLister: &fakes.NodeListerMock{
					GetFunc: func(namespace string, name string) (*apimgmtv3.Node, error) {
						return nil, nil
					},
					ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Node, error) {
						return nil, test.nodeListerErr
					},
				},
				UserMgr:   userManager,
				TokenMgr:  &fakeTokenManager{},
				AuthToken: &fakeAuth,
			}
			err = handler.GenerateKubeconfigActionHandler("not-used", nil, apiContext)
			if test.wantErr {
				assert.Error(t, err, "expected an error but did not get one")
			} else {
				assert.NoError(t, err, "got an error when calling generate kubeconfig")
				assert.Len(t, recorder.Responses, 1, "expected a single response")
				response := recorder.Responses[0]
				assert.Equal(t, response.Code, 200, "expected 200 response code")
				data, ok := response.Data.(map[string]interface{})
				assert.True(t, ok, "type assertion failed")
				kubeconfig, ok := data["config"].(string)
				assert.True(t, ok, "no string kubeconfig in response data")
				if test.generateToken == "true" {
					assert.Contains(t, kubeconfig, fmt.Sprintf("kubeconfig-%s:", testUser), "token expected in kubeconfig but was missing")
				}
				if test.hostname == "" {
					assert.Contains(t, kubeconfig, fakeHost, "expected hostname from request")
				} else {
					assert.Contains(t, kubeconfig, test.hostname, "expected server hostname in kubeconfig")
				}

			}
		})
	}
}

// fakeClusterStore implements types.Store for the purposes of testing
type fakeClusterStore struct {
	err     error
	cluster v3.Cluster
}

func (f *fakeClusterStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	if f.err != nil {
		return nil, f.err
	}
	return convert.EncodeToMap(f.cluster)
}

// The rest of these methods have no functionality, and only serve to implement the types.Store interface
func (f *fakeClusterStore) Context() types.StorageContext { return "" }
func (f *fakeClusterStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeClusterStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeClusterStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeClusterStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, nil
}
func (f *fakeClusterStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, nil
}

// normanRecorder is like httptest.ResponseRecorder, but for norman's types.ResponseWriter interface
type normanRecorder struct {
	Responses []struct {
		Code int
		Data interface{}
	}
}

func (n *normanRecorder) Write(apiContext *types.APIContext, code int, obj interface{}) {
	if n.Responses == nil {
		n.Responses = []struct {
			Code int
			Data interface{}
		}{}
	}
	n.Responses = append(n.Responses, struct {
		Code int
		Data interface{}
	}{
		Code: code,
		Data: obj,
	})
}

const errUserName = "errUser"

// fakeAuthToken implements requests.Authenticator for the purposes of testing
type fakeAuthToken struct {
	token apimgmtv3.Token
	err   error
}

func (f *fakeAuthToken) TokenFromRequest(req *http.Request) (accessor.TokenAccessor, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.token, nil
}

type fakeTokenManager struct{}

func (f *fakeTokenManager) EnsureToken(input user.TokenInput) (string, runtime.Object, error) {
	if input.UserName == errUserName {
		return "", nil, fmt.Errorf("can't generate token for err user")
	}
	return input.TokenName + ":" + "tokenvalue", nil, nil
}
func (f *fakeTokenManager) EnsureClusterToken(clusterName string, input user.TokenInput) (string, runtime.Object, error) {
	if input.UserName == errUserName {
		return "", nil, fmt.Errorf("can't generate token for err user")
	}
	return input.TokenName + ":" + "tokenvalue", nil, nil
}
