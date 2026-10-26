package tokens

import (
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// recordingTokenClient is a fakeTokenClient that records and returns the
// tokens passed to Create.
type recordingTokenClient struct {
	fakeTokenClient
	created []*apiv3.Token
}

func (r *recordingTokenClient) Create(o *apiv3.Token) (*apiv3.Token, error) {
	r.created = append(r.created, o.DeepCopy())
	return o.DeepCopy(), nil
}

func TestAuthProviderForPrincipal(t *testing.T) {
	tests := map[string]struct {
		principal apiv3.Principal
		want      string
	}{
		"principal with provider": {
			principal: apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{Name: "openldap_user://cn=user,dc=example,dc=com"},
				Provider:   "openldap",
			},
			want: "openldap",
		},
		"principal without provider": {
			principal: apiv3.Principal{},
			want:      "local",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.want, authProviderForPrincipal(tt.principal))
		})
	}
}

func TestCreateDerivedTokenAuthProvider(t *testing.T) {
	tests := map[string]struct {
		principal apiv3.Principal
		want      string
	}{
		"derived from principal provider, not parent token AuthProvider": {
			principal: apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{Name: "openldap_user://cn=user,dc=example,dc=com"},
				Provider:   "openldap",
			},
			want: "openldap",
		},
		"local when the parent token has no principal provider": {
			principal: apiv3.Principal{},
			want:      "local",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tokenName := "token-parent"
			tokenKey := mustGenerateRandomToken(t)
			parent := &apiv3.Token{
				ObjectMeta: metav1.ObjectMeta{Name: tokenName},
				Token:      tokenKey,
				UserID:     "u-mo12345",
				// Deliberately differs from the principal's provider so
				// that copying the parent's AuthProvider is detected.
				AuthProvider:  "parent-provider",
				UserPrincipal: tt.principal,
			}
			di := &dummyIndexer{Store: cache.NewStore(cache.MetaNamespaceKeyFunc)}
			di.Empty()
			client := &recordingTokenClient{fakeTokenClient: fakeTokenClient{get: parent}}
			m := Manager{
				tokenIndexer: di,
				tokens:       client,
			}

			derived, unhashedKey, code, err := m.createDerivedToken(clientv3.Token{Description: "derived"}, tokenName+":"+tokenKey)
			require.NoError(t, err)
			assert.Equal(t, 0, code)
			assert.NotEmpty(t, unhashedKey)

			require.Len(t, client.created, 1)
			assert.Equal(t, tt.want, client.created[0].AuthProvider)
			assert.Equal(t, tt.want, derived.AuthProvider)
			assert.Equal(t, tt.principal, derived.UserPrincipal)
			assert.True(t, derived.IsDerived)
		})
	}
}

func TestEnsureClusterTokenAuthProvider(t *testing.T) {
	tests := map[string]struct {
		principal apiv3.Principal
		want      string
	}{
		"principal with provider": {
			principal: apiv3.Principal{
				ObjectMeta: metav1.ObjectMeta{Name: "openldap_user://cn=user,dc=example,dc=com"},
				Provider:   "openldap",
			},
			want: "openldap",
		},
		"local when there is no principal": {
			principal: apiv3.Principal{},
			want:      "local",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client := &recordingTokenClient{}
			m := Manager{tokens: client}

			fullKey, obj, err := m.EnsureClusterToken("c-abcde", user.TokenInput{
				TokenName:     "kubeconfig-u-mo12345",
				Description:   "cluster token",
				Kind:          "kubeconfig",
				UserName:      "u-mo12345",
				Randomize:     true,
				UserPrincipal: tt.principal,
			})
			require.NoError(t, err)
			assert.NotEmpty(t, fullKey)

			require.Len(t, client.created, 1)
			assert.Equal(t, tt.want, client.created[0].AuthProvider)
			assert.Equal(t, tt.principal, client.created[0].UserPrincipal)
			assert.Equal(t, "c-abcde", client.created[0].ClusterName)

			created, ok := obj.(*apiv3.Token)
			require.True(t, ok)
			assert.Equal(t, tt.want, created.AuthProvider)
		})
	}
}
