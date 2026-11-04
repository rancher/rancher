package activedirectory

import (
	"net/http"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
)

func TestADProviderAuthenticateUserResolvesConfigName(t *testing.T) {
	type servers struct {
		activedirectory, adCorp, openldap *authconfigtest.LDAPServer
	}

	tests := map[string]struct {
		configName string
		enabled    map[string]bool
		// wantDialed returns the directory server that the login should
		// connect to, or nil if the login should be rejected.
		wantDialed  func(servers) *authconfigtest.LDAPServer
		wantStatus  int
		wantMessage string
	}{
		"type-only login uses the only enabled config": {
			enabled:    map[string]bool{"ad-corp": true, "openldap": true},
			wantDialed: func(s servers) *authconfigtest.LDAPServer { return s.adCorp },
		},
		"type-only login with no enabled config is rejected": {
			enabled:     map[string]bool{"openldap": true},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "no auth config is enabled for provider activedirectory, specify configName",
		},
		"type-only login with several enabled configs is rejected": {
			enabled:     map[string]bool{"activedirectory": true, "ad-corp": true, "openldap": true},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "several auth configs are enabled for provider activedirectory, specify configName as one of: activedirectory, ad-corp",
		},
		"login with configName uses that config": {
			configName: "ad-corp",
			enabled:    map[string]bool{"activedirectory": true, "ad-corp": true},
			wantDialed: func(s servers) *authconfigtest.LDAPServer { return s.adCorp },
		},
		"login with configName of a disabled config is rejected": {
			configName:  "activedirectory",
			enabled:     map[string]bool{"ad-corp": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"login with configName of an unknown config is rejected": {
			configName:  "ad-missing",
			enabled:     map[string]bool{"activedirectory": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
		"login with configName of a config of another provider is rejected": {
			configName:  "openldap",
			enabled:     map[string]bool{"activedirectory": true, "openldap": true},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "authentication failed",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := servers{
				activedirectory: authconfigtest.NewLDAPServer(t),
				adCorp:          authconfigtest.NewLDAPServer(t),
				openldap:        authconfigtest.NewLDAPServer(t),
			}
			authConfigs, authConfigLister := authconfigtest.New(t,
				adAuthConfig("activedirectory", client.ActiveDirectoryConfigType, tt.enabled["activedirectory"], s.activedirectory),
				adAuthConfig("ad-corp", client.ActiveDirectoryConfigType, tt.enabled["ad-corp"], s.adCorp),
				adAuthConfig("openldap", client.OpenLdapConfigType, tt.enabled["openldap"], s.openldap),
			)
			p := &adProvider{authConfigs: authConfigs, authConfigLister: authConfigLister}

			_, _, _, err := p.AuthenticateUser(nil, nil, &v3.BasicLogin{
				GenericLogin: v3.GenericLogin{Type: publicclient.ActiveDirectoryProviderType, Name: ProviderName, ConfigName: tt.configName},
				Username:     "testuser1",
				Password:     "testuser1",
			})

			all := []*authconfigtest.LDAPServer{s.activedirectory, s.adCorp, s.openldap}
			if tt.wantDialed == nil {
				authconfigtest.RequireLoginRejected(t, err, tt.wantStatus, tt.wantMessage)
				authconfigtest.RequireNotDialed(t, all...)
				return
			}

			want := tt.wantDialed(s)
			authconfigtest.RequireDialed(t, want)
			var others []*authconfigtest.LDAPServer
			for _, srv := range all {
				if srv != want {
					others = append(others, srv)
				}
			}
			authconfigtest.RequireNotDialed(t, others...)
		})
	}
}

func adAuthConfig(name, configType string, enabled bool, srv *authconfigtest.LDAPServer) map[string]any {
	return map[string]any{
		"metadata":          map[string]any{"name": name},
		"type":              configType,
		"enabled":           enabled,
		"servers":           []string{srv.Host},
		"port":              srv.Port,
		"connectionTimeout": 1000,
	}
}
