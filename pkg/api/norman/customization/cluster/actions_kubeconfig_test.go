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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
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

// restoreSettings restores the current values of the given settings when the test ends.
func restoreSettings(t *testing.T, list ...settings.Setting) {
	t.Helper()

	for _, setting := range list {
		value := setting.Get()
		t.Cleanup(func() {
			require.NoError(t, setting.Set(value))
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

func TestGenerateKubeconfigActionHandlerExecUser(t *testing.T) {
	const (
		testClusterName = "test-cluster"
		testUserID      = "u-w7drc"
		fakeHost        = "fake-request-host.fake"
	)

	tests := []struct {
		name              string
		execGetToken      string
		generateToken     string
		clusterAceEnabled bool
		authProvider      string
		tokenFetchErr     error

		wantErr   bool
		wantToken string
		wantArgs  []string
	}{
		{
			name:          "token command with setting off",
			execGetToken:  "false",
			generateToken: "false",
			authProvider:  "github",
			wantArgs:      []string{"token", "--server=" + fakeHost, "--user=" + testClusterName},
		},
		{
			name:          "token command with setting off ignores token fetch error",
			execGetToken:  "false",
			generateToken: "false",
			tokenFetchErr: fmt.Errorf("token not found"),
			wantArgs:      []string{"token", "--server=" + fakeHost, "--user=" + testClusterName},
		},
		{
			name:          "get-token command",
			execGetToken:  "true",
			generateToken: "false",
			authProvider:  "github",
			wantArgs:      []string{"auth", "get-token", "--server=" + fakeHost, "--user-id=" + testUserID, "--auth-provider=github"},
		},
		{
			name:              "get-token command with authorized cluster endpoint",
			execGetToken:      "true",
			generateToken:     "false",
			clusterAceEnabled: true,
			authProvider:      "github",
			wantArgs:          []string{"auth", "get-token", "--server=" + fakeHost, "--cluster=" + testClusterName, "--user-id=" + testUserID, "--auth-provider=github"},
		},
		{
			name:          "get-token command without auth provider",
			execGetToken:  "true",
			generateToken: "false",
			wantArgs:      []string{"auth", "get-token", "--server=" + fakeHost, "--user-id=" + testUserID},
		},
		{
			name:          "get-token command fails on token fetch error",
			execGetToken:  "true",
			generateToken: "false",
			tokenFetchErr: fmt.Errorf("token not found"),
			wantErr:       true,
		},
		{
			name:          "embedded token with setting on",
			execGetToken:  "true",
			generateToken: "true",
			authProvider:  "github",
			wantToken:     "kubeconfig-" + testUserID + ":tokenvalue",
		},
		{
			name:              "embedded token with setting on and authorized cluster endpoint",
			execGetToken:      "true",
			generateToken:     "true",
			clusterAceEnabled: true,
			authProvider:      "github",
			wantToken:         "kubeconfig-" + testUserID + ":tokenvalue",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restoreSettings(t, settings.KubeconfigExecGetToken, settings.KubeconfigGenerateToken, settings.ServerURL)
			require.NoError(t, settings.KubeconfigExecGetToken.Set(test.execGetToken))
			require.NoError(t, settings.KubeconfigGenerateToken.Set(test.generateToken))
			require.NoError(t, settings.ServerURL.Set(""))

			ctrl := gomock.NewController(t)
			userManager := userMocks.NewMockManager(ctrl)
			userManager.EXPECT().GetUser(gomock.Any()).Return(testUserID).AnyTimes()

			cluster := v3.Cluster{Name: testClusterName}
			if test.clusterAceEnabled {
				cluster.LocalClusterAuthEndpoint = &v3.LocalClusterAuthEndpoint{Enabled: true}
			}

			testSchemas := types.NewSchemas().AddSchemas(managementSchema.Schemas)
			testSchemas.Schema(&managementSchema.Version, v3.ClusterType).Store = &fakeClusterStore{cluster: cluster}

			recorder := normanRecorder{}
			apiContext := &types.APIContext{
				ID:             testClusterName,
				Version:        &managementSchema.Version,
				Type:           v3.ClusterType,
				ResponseWriter: &recorder,
				Schemas:        testSchemas,
				Request:        &http.Request{Host: fakeHost},
			}

			handler := ActionHandler{
				NodeLister: &fakes.NodeListerMock{
					ListFunc: func(namespace string, selector labels.Selector) ([]*apimgmtv3.Node, error) {
						return nil, nil
					},
				},
				UserMgr:  userManager,
				TokenMgr: &fakeTokenManager{},
				AuthToken: &fakeAuthToken{
					token: apimgmtv3.Token{
						UserID:       "u-tokenuser", // The user id must come from the request user, not the token.
						AuthProvider: test.authProvider,
					},
					err: test.tokenFetchErr,
				},
			}

			err := handler.GenerateKubeconfigActionHandler("not-used", nil, apiContext)
			if test.wantErr {
				require.ErrorIs(t, err, test.tokenFetchErr)
				assert.Empty(t, recorder.Responses)
				return
			}
			require.NoError(t, err)

			require.Len(t, recorder.Responses, 1)
			require.Equal(t, http.StatusOK, recorder.Responses[0].Code)
			data, ok := recorder.Responses[0].Data.(map[string]interface{})
			require.True(t, ok)
			content, ok := data["config"].(string)
			require.True(t, ok)

			config, err := clientcmd.Load([]byte(content))
			require.NoError(t, err)
			require.Len(t, config.AuthInfos, 1)
			authInfo := config.AuthInfos[testClusterName]
			require.NotNil(t, authInfo)

			if test.wantToken != "" {
				assert.Equal(t, test.wantToken, authInfo.Token)
				assert.Nil(t, authInfo.Exec)
				return
			}

			assert.Empty(t, authInfo.Token)
			require.NotNil(t, authInfo.Exec)
			assert.Equal(t, "rancher", authInfo.Exec.Command)
			assert.Equal(t, test.wantArgs, authInfo.Exec.Args)
		})
	}
}
