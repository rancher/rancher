package activedirectory

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestADProviderGetPrincipalUsesTokenConfig(t *testing.T) {
	tests := map[string]struct {
		principalID string
		wantFetched []string
		wantErr     string
	}{
		"principal from the token's config": {
			principalID: "corp-ad_user://cn=bob,dc=corp,dc=example,dc=com",
			wantFetched: []string{"corp-ad"},
		},
		"principal from another config": {
			principalID: "it-ad_user://cn=bob,dc=it,dc=example,dc=com",
			wantErr:     "not issued by",
		},
		"group principal from another config": {
			principalID: "it-ad_group://cn=admins,dc=it,dc=example,dc=com",
			wantErr:     "not issued by",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			authConfigs, fetched := authconfigtest.NewRecordingClient(t)
			p := &adProvider{
				authConfigs: authConfigs,
			}
			token := &v3.Token{
				UserPrincipal: v3.Principal{
					ObjectMeta: metav1.ObjectMeta{Name: "corp-ad_user://alice"},
				},
			}

			_, err := p.GetPrincipal(tt.principalID, token)
			// Lookups for the token's config fail because no LDAP server is
			// configured. The success path can't be exercised here because
			// adProvider.ldapConnection always dials a real LDAP server; it
			// is not injectable with the ldap fakes.
			require.Error(t, err)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			}
			assert.Equal(t, tt.wantFetched, fetched())
		})
	}
}
