package ldap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"reflect"
	"testing"
	"time"

	ldapv3 "github.com/go-ldap/ldap/v3"
	"github.com/pkg/errors"

	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

const (
	DummySAUsername     = "satestuser1"
	DummySAUPassword    = "sapassword123"
	UserObjectClassName = "inetOrgPerson"
)

func Test_ldapProvider_loginUser(t *testing.T) {
	type fields struct {
		ctx                   context.Context
		authConfigs           v3.AuthConfigInterface
		secrets               corev1.SecretInterface
		userMGR               mockUserManager
		tokenMGR              *tokens.Manager
		certs                 string
		caPool                *x509.CertPool
		providerName          string
		testAndApplyInputType string
		userScope             string
		groupScope            string
	}
	type args struct {
		lConn      ldapv3.Client
		credential *v32.BasicLogin
		config     *v3.LdapConfig
		caPool     *x509.CertPool
	}
	tests := []struct {
		name                string
		fields              fields
		args                args
		wantUserPrincipal   v3.Principal
		wantGroupPrincipals []v3.Principal
		wantErr             bool
	}{
		{
			name: "successful user login",
			fields: fields{
				userMGR: mockUserManager{
					hasAccess: true,
				},
				tokenMGR:   &tokens.Manager{},
				caPool:     &x509.CertPool{},
				userScope:  "providername_user",
				groupScope: "providername_group",
			},
			args: args{
				lConn: newMockLdapConnClient(),
				credential: &v32.BasicLogin{
					Username: DummyUsername,
					Password: DummyPassword,
				},
				config: &v3.LdapConfig{
					LdapFields: v32.LdapFields{
						ServiceAccountDistinguishedName: DummySAUsername,
						ServiceAccountPassword:          DummySAUPassword,
						UserObjectClass:                 UserObjectClassName,
					},
				},
				caPool: &x509.CertPool{},
			},
			wantUserPrincipal: v3.Principal{
				ObjectMeta: v1.ObjectMeta{
					Name: "providername_user://ldap.test.domain",
				},
				PrincipalType: "user",
				Me:            true,
			},
			wantGroupPrincipals: []v3.Principal{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "providername_group://ldap.test.domain",
					},
					PrincipalType: "user",
					Me:            true,
				},
			},
			wantErr: false,
		},
		{
			name: "user login with invalid credentials",
			fields: fields{
				userMGR: mockUserManager{
					hasAccess: false,
				},
				tokenMGR:   &tokens.Manager{},
				caPool:     &x509.CertPool{},
				userScope:  "providername_user",
				groupScope: "providername_group",
			},
			args: args{
				lConn: &mockLdapConn{
					canAuthenticate: false,
				},
				credential: &v32.BasicLogin{
					Username: DummyUsername,
					Password: DummyPassword,
				},
				config: &v3.LdapConfig{
					LdapFields: v32.LdapFields{
						ServiceAccountDistinguishedName: DummySAUsername,
						ServiceAccountPassword:          DummySAUPassword,
						UserObjectClass:                 UserObjectClassName,
					},
				},
				caPool: &x509.CertPool{},
			},
			wantUserPrincipal:   v3.Principal{},
			wantGroupPrincipals: nil,
			wantErr:             true,
		},
		{
			name: "user login without access permissions",
			fields: fields{
				userMGR: mockUserManager{
					hasAccess: false,
				},
				tokenMGR:   &tokens.Manager{},
				caPool:     &x509.CertPool{},
				userScope:  "providername_user",
				groupScope: "providername_group",
			},
			args: args{
				lConn: newMockLdapConnClient(),
				credential: &v32.BasicLogin{
					Username: DummyUsername,
					Password: DummyPassword,
				},
				config: &v3.LdapConfig{
					LdapFields: v32.LdapFields{
						ServiceAccountDistinguishedName: DummySAUsername,
						ServiceAccountPassword:          DummySAUPassword,
						UserObjectClass:                 UserObjectClassName,
					},
				},
				caPool: &x509.CertPool{},
			},
			wantUserPrincipal:   v3.Principal{},
			wantGroupPrincipals: nil,
			wantErr:             true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ldapProvider{
				ctx:                   tt.fields.ctx,
				authConfigs:           tt.fields.authConfigs,
				secrets:               tt.fields.secrets,
				userMGR:               tt.fields.userMGR,
				tokenMGR:              tt.fields.tokenMGR,
				certs:                 tt.fields.certs,
				caPool:                tt.fields.caPool,
				providerName:          tt.fields.providerName,
				testAndApplyInputType: tt.fields.testAndApplyInputType,
				userScope:             tt.fields.userScope,
				groupScope:            tt.fields.groupScope,
			}
			gotUserPrincipal, gotGroupPrincipals, err := p.loginUser(tt.args.lConn, tt.args.credential, tt.args.config, tt.args.caPool)
			if (err != nil) != tt.wantErr {
				t.Errorf("ldapProvider.loginUser() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotUserPrincipal, tt.wantUserPrincipal) {
				t.Errorf("ldapProvider.loginUser() got = %v, want %v", gotUserPrincipal, tt.wantUserPrincipal)
			}
			if !reflect.DeepEqual(gotGroupPrincipals, tt.wantGroupPrincipals) {
				t.Errorf("ldapProvider.loginUser() got1 = %v, want %v", gotGroupPrincipals, tt.wantGroupPrincipals)
			}
		})
	}
}

type mockLdapConn struct {
	SimpleBindResponse     *ldapv3.SimpleBindResult
	PasswordModifyResponse *ldapv3.PasswordModifyResult
	SearchResponse         *ldapv3.SearchResult
	canAuthenticate        bool
}

func (m mockLdapConn) Start() {
	panic("unimplemented")
}
func (m mockLdapConn) StartTLS(*tls.Config) error {
	panic("unimplemented")
}
func (m mockLdapConn) Close() {
	panic("unimplemented")
}
func (m mockLdapConn) IsClosing() bool {
	panic("unimplemented")
}
func (m mockLdapConn) SetTimeout(time.Duration) {
	panic("unimplemented")
}
func (m mockLdapConn) Bind(username, password string) error {
	if !m.canAuthenticate {
		return ldapv3.NewError(ldapv3.LDAPResultInvalidCredentials, errors.New("ldap: invalid credentials"))
	}
	return nil
}
func (m mockLdapConn) UnauthenticatedBind(username string) error {
	panic("unimplemented")
}
func (m mockLdapConn) SimpleBind(*ldapv3.SimpleBindRequest) (*ldapv3.SimpleBindResult, error) {
	panic("unimplemented")
}
func (m mockLdapConn) ExternalBind() error {
	panic("unimplemented")
}
func (m mockLdapConn) Add(*ldapv3.AddRequest) error {
	panic("unimplemented")
}
func (m mockLdapConn) Del(*ldapv3.DelRequest) error {
	panic("unimplemented")
}
func (m mockLdapConn) Modify(*ldapv3.ModifyRequest) error {
	panic("unimplemented")
}
func (m mockLdapConn) ModifyDN(*ldapv3.ModifyDNRequest) error {
	panic("unimplemented")
}
func (m mockLdapConn) ModifyWithResult(*ldapv3.ModifyRequest) (*ldapv3.ModifyResult, error) {
	panic("unimplemented")
}
func (m mockLdapConn) Compare(dn, attribute, value string) (bool, error) {
	panic("unimplemented")
}
func (m mockLdapConn) PasswordModify(*ldapv3.PasswordModifyRequest) (*ldapv3.PasswordModifyResult, error) {
	panic("unimplemented")
}
func (m mockLdapConn) Search(*ldapv3.SearchRequest) (*ldapv3.SearchResult, error) {
	return m.SearchResponse, nil
}
func (m mockLdapConn) SearchWithPaging(searchRequest *ldapv3.SearchRequest, pagingSize uint32) (*ldapv3.SearchResult, error) {
	return m.SearchResponse, nil
}

func newMockLdapConnClient() *mockLdapConn {
	return &mockLdapConn{
		SimpleBindResponse: &ldapv3.SimpleBindResult{
			Controls: []ldapv3.Control{},
		},
		PasswordModifyResponse: &ldapv3.PasswordModifyResult{
			GeneratedPassword: "",
		},
		SearchResponse: &ldapv3.SearchResult{
			Entries: []*ldapv3.Entry{
				{
					DN: "ldap.test.domain",
					Attributes: []*ldapv3.EntryAttribute{
						{
							Name:   "objectclass",
							Values: []string{UserObjectClassName},
						},
					},
				},
			},
			Referrals: []string{},
			Controls:  []ldapv3.Control{},
		},
		canAuthenticate: true,
	}
}

type mockUserManager struct {
	hasAccess bool
}

func (m mockUserManager) SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) GetUser(apiContext *types.APIContext) string {
	panic("unimplemented")
}
func (m mockUserManager) EnsureToken(input user.TokenInput) (string, error) {
	panic("unimplemented")
}
func (m mockUserManager) EnsureClusterToken(clusterName string, input user.TokenInput) (string, error) {
	panic("unimplemented")
}
func (m mockUserManager) DeleteToken(tokenName string) error {
	panic("unimplemented")
}
func (m mockUserManager) EnsureUser(principalName, displayName string) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []v3.Principal) (bool, error) {
	return m.hasAccess, nil
}
func (m mockUserManager) SetPrincipalOnCurrentUserByUserID(userID string, principal v3.Principal) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error {
	panic("unimplemented")
}
func (m mockUserManager) GetUserByPrincipalID(principalName string) (*v3.User, error) {
	panic("unimplemented")
}
func (m mockUserManager) GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal v3.Principal) (*v3.Token, string, error) {
	panic("unimplemented")
}
