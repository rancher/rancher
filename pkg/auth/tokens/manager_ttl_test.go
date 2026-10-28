package tokens

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rancher/norman/types"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	hourMillis = int64(time.Hour / time.Millisecond)
	dayMillis  = 24 * hourMillis
)

// setSettings sets the given settings for the duration of the test.
func setSettings(t *testing.T, values map[*settings.Setting]string) {
	t.Helper()

	for setting, value := range values {
		prior := setting.Get()
		require.NoError(t, setting.Set(value))
		t.Cleanup(func() {
			assert.NoError(t, setting.Set(prior))
		})
	}
}

// setTokenTTLSettings sets the token and kubeconfig TTL settings, in minutes, for the duration of the test.
func setTokenTTLSettings(t *testing.T, maxMinutes, defaultMinutes, kubeconfigMaxMinutes, kubeconfigDefaultMinutes string) {
	t.Helper()

	setSettings(t, map[*settings.Setting]string{
		&settings.AuthTokenMaxTTLMinutes:           maxMinutes,
		&settings.AuthTokenDefaultTTLMinutes:       defaultMinutes,
		&settings.KubeconfigMaxTokenTTLMinutes:     kubeconfigMaxMinutes,
		&settings.KubeconfigDefaultTokenTTLMinutes: kubeconfigDefaultMinutes,
	})
}

func TestGetKubeconfigTokenTTLs(t *testing.T) {
	tests := []struct {
		name                     string
		maxMinutes               string
		kubeconfigMaxMinutes     string
		kubeconfigDefaultMinutes string
		wantDefault              int64
		wantMax                  int64
	}{
		{
			name:                     "defaults",
			maxMinutes:               "129600",
			kubeconfigMaxMinutes:     "129600",
			kubeconfigDefaultMinutes: "43200",
			wantDefault:              30 * dayMillis,
			wantMax:                  90 * dayMillis,
		},
		{
			name:                     "kubeconfig max above the token max",
			maxMinutes:               "129600",
			kubeconfigMaxMinutes:     "259200",
			kubeconfigDefaultMinutes: "172800",
			wantDefault:              90 * dayMillis,
			wantMax:                  90 * dayMillis,
		},
		{
			name:                     "kubeconfig max below the token max",
			maxMinutes:               "129600",
			kubeconfigMaxMinutes:     "28800",
			kubeconfigDefaultMinutes: "43200",
			wantDefault:              20 * dayMillis,
			wantMax:                  20 * dayMillis,
		},
		{
			name:                     "both kubeconfig settings 0",
			maxMinutes:               "129600",
			kubeconfigMaxMinutes:     "0",
			kubeconfigDefaultMinutes: "0",
			wantDefault:              90 * dayMillis,
			wantMax:                  90 * dayMillis,
		},
		{
			name:                     "no limit at any level",
			maxMinutes:               "0",
			kubeconfigMaxMinutes:     "0",
			kubeconfigDefaultMinutes: "0",
			wantDefault:              -1,
			wantMax:                  -1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setTokenTTLSettings(t, test.maxMinutes, "129600", test.kubeconfigMaxMinutes, test.kubeconfigDefaultMinutes)

			defaultTTL, err := GetKubeconfigDefaultTokenTTLInMilliSeconds()
			require.NoError(t, err)
			assert.Equal(t, test.wantDefault, *defaultTTL, "default")

			maxTTL, err := GetKubeconfigMaxTokenTTLInMilliSeconds()
			require.NoError(t, err)
			assert.Equal(t, test.wantMax, maxTTL, "max")
		})
	}
}

func TestCreateDerivedTokenTTL(t *testing.T) {
	tests := []struct {
		name           string
		maxMinutes     string
		defaultMinutes string
		ttl            int64
		want           int64
	}{
		{
			name:           "default",
			maxMinutes:     "129600",
			defaultMinutes: "43200",
			want:           30 * dayMillis,
		},
		{
			name:           "no max, default 0",
			maxMinutes:     "0",
			defaultMinutes: "0",
			want:           0,
		},
		{
			name:           "never expires, no max",
			maxMinutes:     "0",
			defaultMinutes: "43200",
			ttl:            -1,
			want:           0,
		},
		{
			name:           "never expires, finite max",
			maxMinutes:     "129600",
			defaultMinutes: "43200",
			ttl:            -1,
			want:           90 * dayMillis,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setTokenTTLSettings(t, test.maxMinutes, test.defaultMinutes, "129600", "43200")

			tokenName := "login"
			tokenKey := mustGenerateRandomToken(t)
			tokens := &recordingTokenClient{
				fakeTokenClient: fakeTokenClient{
					get: &apiv3.Token{
						ObjectMeta: metav1.ObjectMeta{Name: tokenName},
						Token:      tokenKey,
						UserID:     "u-mo12345",
					},
				},
			}
			di := &dummyIndexer{Store: cache.NewStore(cache.MetaNamespaceKeyFunc)}
			di.Empty()
			manager := Manager{tokenIndexer: di, tokens: tokens}

			_, _, _, err := manager.createDerivedToken(clientv3.Token{TTLMillis: test.ttl}, tokenName+":"+tokenKey)
			require.NoError(t, err)

			require.Len(t, tokens.created, 1)
			assert.Equal(t, test.want, tokens.created[0].TTLMillis)
		})
	}
}

func TestEnsureClusterTokenNegativeTTL(t *testing.T) {
	tokens := &recordingTokenClient{}
	manager := Manager{tokens: tokens}

	ttl := int64(-1)
	_, _, err := manager.EnsureClusterToken("c-m-abcde", user.TokenInput{
		TokenName: "kubeconfig-u-abcde",
		Kind:      "kubeconfig",
		UserName:  "u-abcde",
		TTL:       &ttl,
		Randomize: true,
	})
	require.NoError(t, err)

	require.Len(t, tokens.created, 1)
	assert.Equal(t, int64(0), tokens.created[0].TTLMillis)
	assert.False(t, tokens.created[0].GetIsExpired())
}

func TestCreateTokenAndSetCookieTTL(t *testing.T) {
	tests := []struct {
		name           string
		sessionMinutes string
		want           int64
		wantErr        bool
	}{
		{
			name:           "session ttl",
			sessionMinutes: "960",
			want:           16 * hourMillis,
		},
		{
			name:           "no session ttl limit",
			sessionMinutes: "0",
			want:           0,
		},
		{
			name:           "session ttl that doesn't parse",
			sessionMinutes: "16h",
			wantErr:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setSettings(t, map[*settings.Setting]string{
				&settings.AuthUserSessionTTLMinutes: test.sessionMinutes,
			})

			tokens := &recordingTokenClient{}
			manager := Manager{tokens: tokens}

			response := httptest.NewRecorder()
			request := &types.APIContext{
				Request:        httptest.NewRequest(http.MethodPost, "https://rancher.example.com/v3/testAndApply", nil),
				Response:       response,
				ResponseWriter: &normanRecorder{},
			}
			principal := apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{Name: "local://u-abcde"},
				Provider:   "local",
			}

			err := manager.CreateTokenAndSetCookie("u-abcde", principal, nil, "", "test", request)
			if test.wantErr {
				require.Error(t, err)
				assert.Empty(t, tokens.created)
				assert.Empty(t, response.Result().Cookies())
				return
			}
			require.NoError(t, err)

			require.Len(t, tokens.created, 1)
			assert.Equal(t, test.want, tokens.created[0].TTLMillis)
			assert.NotEmpty(t, response.Result().Cookies())
		})
	}
}
