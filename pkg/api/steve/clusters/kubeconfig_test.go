package clusters

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rancher/apiserver/pkg/types"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8suser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/tools/clientcmd"
)

type allowGet struct {
	types.AccessControl
}

func (allowGet) CanGet(*types.APIRequest, *types.APISchema) error { return nil }

type responseRecorder struct {
	code int
	obj  types.APIObject
}

func (r *responseRecorder) Write(_ *types.APIRequest, code int, obj types.APIObject) {
	r.code = code
	r.obj = obj
}

func (r *responseRecorder) WriteList(*types.APIRequest, int, types.APIObjectList) {}

type fakeAuthToken struct {
	token apimgmtv3.Token
	err   error
}

func (f *fakeAuthToken) TokenFromRequest(*http.Request) (accessor.TokenAccessor, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.token, nil
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

func TestKubeconfigDownload(t *testing.T) {
	const (
		testClusterID = "c-m-abc12"
		testUserID    = "u-w7drc"
		fakeHost      = "fake-request-host.fake"
	)

	tests := []struct {
		name          string
		execGetToken  string
		authProvider  string
		tokenFetchErr error

		wantErr  bool
		wantArgs []string
	}{
		{
			name:         "token command with setting off",
			execGetToken: "false",
			authProvider: "github",
			wantArgs:     []string{"token", "--server=" + fakeHost, "--user=" + testClusterID},
		},
		{
			name:          "token command with setting off ignores token fetch error",
			execGetToken:  "false",
			tokenFetchErr: errors.New("token not found"),
			wantArgs:      []string{"token", "--server=" + fakeHost, "--user=" + testClusterID},
		},
		{
			name:         "get-token command",
			execGetToken: "true",
			authProvider: "github",
			wantArgs:     []string{"auth", "get-token", "--server=" + fakeHost, "--user-id=" + testUserID, "--auth-provider=github"},
		},
		{
			name:         "get-token command without auth provider",
			execGetToken: "true",
			wantArgs:     []string{"auth", "get-token", "--server=" + fakeHost, "--user-id=" + testUserID},
		},
		{
			name:          "get-token command fails on token fetch error",
			execGetToken:  "true",
			tokenFetchErr: errors.New("token not found"),
			wantErr:       true,
		},
	}

	mcmEnabled := features.MCM.Enabled()
	features.MCM.Set(false)
	t.Cleanup(func() { features.MCM.Set(mcmEnabled) })

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restoreSettings(t, settings.KubeconfigExecGetToken, settings.KubeconfigGenerateToken, settings.ServerURL)
			require.NoError(t, settings.KubeconfigExecGetToken.Set(test.execGetToken))
			require.NoError(t, settings.KubeconfigGenerateToken.Set("false"))
			require.NoError(t, settings.ServerURL.Set(""))

			req := httptest.NewRequestWithContext(
				request.WithUser(t.Context(), &k8suser.DefaultInfo{Name: testUserID}),
				http.MethodGet, "/v1/management.cattle.io.clusters/"+testClusterID+"?link=kubeconfig", nil)
			req.Host = fakeHost

			recorder := &responseRecorder{}
			var writeErr error
			apiRequest := types.StoreAPIContext(&types.APIRequest{
				Name:           testClusterID,
				Request:        req,
				Response:       httptest.NewRecorder(),
				ResponseWriter: recorder,
				AccessControl:  allowGet{},
				ErrorHandler:   func(_ *types.APIRequest, err error) { writeErr = err },
			})

			handler := kubeconfigDownload{
				authToken: &fakeAuthToken{
					token: apimgmtv3.Token{UserID: "u-tokenuser", AuthProvider: test.authProvider}, // The user id must come from the request user, not the token.
					err:   test.tokenFetchErr,
				},
			}
			handler.ServeHTTP(httptest.NewRecorder(), apiRequest.Request)

			if test.wantErr {
				require.ErrorIs(t, writeErr, test.tokenFetchErr)
				assert.Zero(t, recorder.code)
				return
			}
			require.NoError(t, writeErr)
			require.Equal(t, http.StatusOK, recorder.code)

			output, ok := recorder.obj.Object.(*GenerateKubeconfigOutput)
			require.True(t, ok)

			config, err := clientcmd.Load([]byte(output.Config))
			require.NoError(t, err)
			require.Len(t, config.AuthInfos, 1)
			authInfo := config.AuthInfos[testClusterID]
			require.NotNil(t, authInfo)
			assert.Empty(t, authInfo.Token)
			require.NotNil(t, authInfo.Exec)
			assert.Equal(t, "rancher", authInfo.Exec.Command)
			assert.Equal(t, test.wantArgs, authInfo.Exec.Args)
		})
	}
}
