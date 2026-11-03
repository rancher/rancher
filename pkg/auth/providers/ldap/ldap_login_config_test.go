package ldap

import (
	"net/http"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/authconfigtest"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	publicclient "github.com/rancher/rancher/pkg/client/generated/management/v3public"
)

func TestLDAPProviderAuthenticateUserResolvesConfigName(t *testing.T) {
	type servers struct {
		openldap, openldapCorp, freeipa, freeipaCorp *authconfigtest.LDAPServer
	}

	loginTypes := map[string]string{
		OpenLdapName: publicclient.OpenLdapProviderType,
		FreeIpaName:  publicclient.FreeIpaProviderType,
	}

	tests := map[string]struct {
		providerName string
		configName   string
		enabled      map[string]bool
		// wantDialed returns the directory server that the login should
		// connect to, or nil if the login should be rejected.
		wantDialed  func(servers) *authconfigtest.LDAPServer
		wantStatus  int
		wantMessage string
	}{
		"openldap type-only login uses the only enabled openldap config": {
			providerName: OpenLdapName,
			enabled:      map[string]bool{"openldap-corp": true, "freeipa": true, "freeipa-corp": true},
			wantDialed:   func(s servers) *authconfigtest.LDAPServer { return s.openldapCorp },
		},
		"freeipa type-only login uses the only enabled freeipa config": {
			providerName: FreeIpaName,
			enabled:      map[string]bool{"freeipa-corp": true, "openldap": true, "openldap-corp": true},
			wantDialed:   func(s servers) *authconfigtest.LDAPServer { return s.freeipaCorp },
		},
		"type-only login with no enabled config of the type is rejected": {
			providerName: OpenLdapName,
			enabled:      map[string]bool{"freeipa": true, "freeipa-corp": true},
			wantStatus:   http.StatusUnprocessableEntity,
			wantMessage:  "no auth config is enabled for provider openldap, specify configName",
		},
		"type-only login with several enabled configs of the type is rejected": {
			providerName: OpenLdapName,
			enabled:      map[string]bool{"openldap": true, "openldap-corp": true, "freeipa": true},
			wantStatus:   http.StatusUnprocessableEntity,
			wantMessage:  "several auth configs are enabled for provider openldap, specify configName as one of: openldap, openldap-corp",
		},
		"login with configName uses that config": {
			providerName: OpenLdapName,
			configName:   "openldap-corp",
			enabled:      map[string]bool{"openldap": true, "openldap-corp": true},
			wantDialed:   func(s servers) *authconfigtest.LDAPServer { return s.openldapCorp },
		},
		"login with configName of a disabled config is rejected": {
			providerName: OpenLdapName,
			configName:   "openldap",
			enabled:      map[string]bool{"openldap-corp": true},
			wantStatus:   http.StatusUnauthorized,
			wantMessage:  "authentication failed",
		},
		"login with configName of an unknown config is rejected": {
			providerName: OpenLdapName,
			configName:   "openldap-missing",
			enabled:      map[string]bool{"openldap": true},
			wantStatus:   http.StatusUnauthorized,
			wantMessage:  "authentication failed",
		},
		"login with configName of a config of another type is rejected": {
			providerName: OpenLdapName,
			configName:   "freeipa",
			enabled:      map[string]bool{"openldap": true, "freeipa": true},
			wantStatus:   http.StatusUnauthorized,
			wantMessage:  "authentication failed",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := servers{
				openldap:     authconfigtest.NewLDAPServer(t),
				openldapCorp: authconfigtest.NewLDAPServer(t),
				freeipa:      authconfigtest.NewLDAPServer(t),
				freeipaCorp:  authconfigtest.NewLDAPServer(t),
			}
			authConfigs, authConfigLister := authconfigtest.New(t,
				ldapAuthConfig("openldap", client.OpenLdapConfigType, tt.enabled["openldap"], s.openldap),
				ldapAuthConfig("openldap-corp", client.OpenLdapConfigType, tt.enabled["openldap-corp"], s.openldapCorp),
				ldapAuthConfig("freeipa", client.FreeIpaConfigType, tt.enabled["freeipa"], s.freeipa),
				ldapAuthConfig("freeipa-corp", client.FreeIpaConfigType, tt.enabled["freeipa-corp"], s.freeipaCorp),
			)
			p := &ldapProvider{
				authConfigs:      authConfigs,
				authConfigLister: authConfigLister,
				providerName:     tt.providerName,
			}

			_, _, _, err := p.AuthenticateUser(nil, nil, &v3.BasicLogin{
				GenericLogin: v3.GenericLogin{Type: loginTypes[tt.providerName], Name: tt.providerName, ConfigName: tt.configName},
				Username:     DummyUsername,
				Password:     DummyPassword,
			})

			all := []*authconfigtest.LDAPServer{s.openldap, s.openldapCorp, s.freeipa, s.freeipaCorp}
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

func ldapAuthConfig(name, configType string, enabled bool, srv *authconfigtest.LDAPServer) map[string]any {
	return map[string]any{
		"metadata":          map[string]any{"name": name},
		"type":              configType,
		"enabled":           enabled,
		"servers":           []string{srv.Host},
		"port":              srv.Port,
		"connectionTimeout": 1000,
	}
}
