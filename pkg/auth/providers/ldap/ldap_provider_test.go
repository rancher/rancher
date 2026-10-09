package ldap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/rancher/norman/objectclient"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/user"
	wcorev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

var (
	DummyCerts    = "dummycerts"
	DummyUsername = "testuser1"
	DummyPassword = "testuser1"
)

func TestGetBasicLogin(t *testing.T) {
	type args struct {
		input any
	}
	tests := []struct {
		name      string
		args      args
		wantLogin *v3.BasicLogin
		wantErr   bool
	}{
		{
			name: "good input credentials",
			args: args{
				input: &v3.BasicLogin{
					Username: DummyUsername,
					Password: DummyPassword,
				},
			},
			wantLogin: &v3.BasicLogin{
				Username: DummyUsername,
				Password: DummyPassword,
			},
			wantErr: false,
		},
		{
			name: "bad input credentials",
			args: args{
				input: "badinput",
			},
			wantLogin: &v3.BasicLogin{},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLogin, err := toBasicLogin(tt.args.input)
			if err != nil {
				if tt.wantErr {
					assert.Errorf(t, err, "unexpected input type")
				} else {
					t.Errorf("toBasicLogin() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if !reflect.DeepEqual(gotLogin, tt.wantLogin) {
				t.Errorf("toBasicLogin() = %v, want %v", gotLogin, tt.wantLogin)
			}
		})
	}
}

func TestLdapProviderGetLDAPConfig(t *testing.T) {
	type fields struct {
		secrets               wcorev1.SecretController
		userMGR               user.Manager
		tokenMGR              *tokens.Manager
		providerName          string
		testAndApplyInputType string
	}
	tests := []struct {
		name                 string
		objectMap            map[string]any
		fields               fields
		wantStoredLdapConfig *v3.LdapConfig
		wantErr              bool
	}{
		{
			name:   "get LDAP config object",
			fields: fields{},
			wantStoredLdapConfig: &v3.LdapConfig{
				LdapFields: v3.LdapFields{
					Certificate: DummyCerts,
				},
			},
			objectMap: map[string]any{
				"Certificate": DummyCerts,
			},
			wantErr: false,
		},
		{
			name: "ldap config is nil",
			fields: fields{
				providerName: "okta",
			},
			objectMap: map[string]any{
				"openLdapConfig": nil,
			},
			wantErr: true,
		},
		{
			name: "ldap config not found",
			fields: fields{
				providerName: "okta",
			},
			objectMap: map[string]any{},
			wantErr:   true,
		},
		{
			name: "no servers found",
			fields: fields{
				providerName: "okta",
			},
			objectMap: map[string]any{
				"openLdapConfig": map[string]any{
					"servers": []string{},
				},
			},
			wantStoredLdapConfig: &v3.LdapConfig{
				LdapFields: v3.LdapFields{
					Servers: []string{},
				},
			},
			wantErr: true,
		},
		{
			name: "server gets added",
			fields: fields{
				providerName: "okta",
			},
			objectMap: map[string]any{
				"openLdapConfig": map[string]any{
					"Certificate": DummyCerts,
					"servers":     []string{"server1"},
				},
			},
			wantStoredLdapConfig: &v3.LdapConfig{
				// LDAP configs nested in SAML configs take the name of the
				// SAML config.
				AuthConfig: v3.AuthConfig{ObjectMeta: metav1.ObjectMeta{Name: "okta"}},
				LdapFields: v3.LdapFields{
					Servers:     []string{"server1"},
					Certificate: DummyCerts,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mockGenericClient{ObjectMap: tt.objectMap}
			p := &ldapProvider{
				secrets:               tt.fields.secrets,
				userMGR:               tt.fields.userMGR,
				tokenMGR:              tt.fields.tokenMGR,
				providerName:          tt.fields.providerName,
				testAndApplyInputType: tt.fields.testAndApplyInputType,
			}
			gotStoredLdapConfig, gotCaPool, err := p.getLDAPConfig(m, tt.fields.providerName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ldapProvider.getLDAPConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotStoredLdapConfig, tt.wantStoredLdapConfig) {
				t.Errorf("ldapProvider.getLDAPConfig() got ldapConfig = %v, want %v", gotStoredLdapConfig, tt.wantStoredLdapConfig)
			}
			if tt.wantErr {
				assert.Nil(t, gotCaPool)
			} else {
				assert.NotNil(t, gotCaPool)
			}
		})
	}
}

func TestLdapProviderGetLDAPConfigUsesConfigCertificate(t *testing.T) {
	makeCertificate := func(commonName string) (string, []byte) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}

		certificate := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: commonName},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign,
		}
		der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		parsedCertificate, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatal(err)
		}

		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), parsedCertificate.RawSubject
	}
	containsSubject := func(pool *x509.CertPool, expected []byte) bool {
		return slices.ContainsFunc(pool.Subjects(), func(subject []byte) bool {
			return slices.Equal(subject, expected)
		})
	}

	firstCertificate, firstSubject := makeCertificate("first config CA")
	secondCertificate, secondSubject := makeCertificate("second config CA")
	provider := &ldapProvider{}

	type configRequest struct {
		name        string
		certificate string
		subject     []byte
		other       []byte
	}
	type configResult struct {
		request           configRequest
		configCertificate string
		pool              *x509.CertPool
		err               error
	}
	requests := []configRequest{
		{name: "first", certificate: firstCertificate, subject: firstSubject, other: secondSubject},
		{name: "second", certificate: secondCertificate, subject: secondSubject, other: firstSubject},
	}
	const lookupsPerConfig = 16
	start := make(chan struct{})
	results := make(chan configResult, len(requests)*lookupsPerConfig)
	var workers sync.WaitGroup
	for _, request := range requests {
		for range lookupsPerConfig {
			workers.Add(1)
			go func(request configRequest) {
				defer workers.Done()
				<-start
				config, pool, err := provider.getLDAPConfig(mockGenericClient{ObjectMap: map[string]any{
					"Certificate": request.certificate,
				}}, request.name)
				result := configResult{request: request, pool: pool, err: err}
				if config != nil {
					result.configCertificate = config.Certificate
				}
				results <- result
			}(request)
		}
	}
	close(start)
	workers.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			t.Errorf("getLDAPConfig(%q) error = %v", result.request.name, result.err)
			continue
		}
		assert.Equal(t, result.request.certificate, result.configCertificate)
		assert.True(t, containsSubject(result.pool, result.request.subject))
		assert.False(t, containsSubject(result.pool, result.request.other))
	}
}

func TestSamlSearchProvider(t *testing.T) {
	// Test in prime environment
	features.ADFSLDAPSearch.Set(true)
	t.Cleanup(features.ADFSLDAPSearch.Unset)
	t.Setenv("RANCHER_VERSION_TYPE", "prime")

	for _, tt := range []struct {
		name    string
		hasLDAP bool
	}{
		{name: "adfs", hasLDAP: true},
		{name: "okta", hasLDAP: true},
		{name: "other", hasLDAP: false},
		{name: "shibboleth", hasLDAP: true},
	} {
		t.Run(""+tt.name, func(t *testing.T) {
			p := ldapProvider{providerName: tt.name}
			assert.Equal(t, tt.hasLDAP, p.samlSearchProvider())
		})
	}
}

func TestSamlSearchProviderNonPrime(t *testing.T) {
	// Test in non-prime environment
	t.Setenv("RANCHER_VERSION_TYPE", "")

	for _, tt := range []struct {
		name    string
		hasLDAP bool
	}{
		{name: "adfs", hasLDAP: false},
		{name: "okta", hasLDAP: true},
		{name: "other", hasLDAP: false},
		{name: "shibboleth", hasLDAP: true},
	} {
		t.Run(""+tt.name, func(t *testing.T) {
			p := ldapProvider{providerName: tt.name}
			assert.Equal(t, tt.hasLDAP, p.samlSearchProvider())
		})
	}
}

type mockGenericClient struct {
	ObjectMap map[string]any
}

func (m mockGenericClient) UnstructuredClient() objectclient.GenericClient {
	panic("unimplemented")
}
func (m mockGenericClient) GroupVersionKind() schema.GroupVersionKind {
	panic("unimplemented")
}
func (m mockGenericClient) Create(o runtime.Object) (runtime.Object, error) {
	panic("unimplemented")
}
func (m mockGenericClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
	panic("unimplemented")
}
func (m mockGenericClient) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	u := &unstructured.Unstructured{
		Object: m.ObjectMap,
	}
	return u, nil
}
func (m mockGenericClient) Update(name string, o runtime.Object) (runtime.Object, error) {
	panic("unimplemented")
}
func (m mockGenericClient) UpdateStatus(name string, o runtime.Object) (runtime.Object, error) {
	panic("unimplemented")
}
func (m mockGenericClient) DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error {
	panic("unimplemented")
}
func (m mockGenericClient) Delete(name string, opts *metav1.DeleteOptions) error {
	panic("unimplemented")
}
func (m mockGenericClient) List(opts metav1.ListOptions) (runtime.Object, error) {
	panic("unimplemented")
}
func (m mockGenericClient) ListNamespaced(namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	panic("unimplemented")
}
func (m mockGenericClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("unimplemented")
}
func (m mockGenericClient) DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("unimplemented")
}
func (m mockGenericClient) Patch(name string, o runtime.Object, patchType types.PatchType, data []byte, subresources ...string) (runtime.Object, error) {
	panic("unimplemented")
}
func (m mockGenericClient) ObjectFactory() objectclient.ObjectFactory {
	panic("unimplemented")
}

func TestGetDNAndScopeFromPrincipalID(t *testing.T) {
	p := &ldapProvider{}
	tests := []struct {
		name        string
		principalID string
		wantDN      string
		wantScope   string
		wantErr     bool
	}{
		{
			name:        "valid user principal",
			principalID: "openldap_user://cn=alice,dc=example,dc=com",
			wantDN:      "cn=alice,dc=example,dc=com",
			wantScope:   "openldap_user",
		},
		{
			name:        "valid group principal",
			principalID: "freeipa_group://cn=admins,cn=groups,dc=example,dc=com",
			wantDN:      "cn=admins,cn=groups,dc=example,dc=com",
			wantScope:   "freeipa_group",
		},
		{
			name:        "principal with colons in DN",
			principalID: "openldap_user://uid=bob:special,dc=example,dc=com",
			wantDN:      "uid=bob:special,dc=example,dc=com",
			wantScope:   "openldap_user",
		},
		{
			name:        "missing colon separator",
			principalID: "openldap_user//cn=alice,dc=example,dc=com",
			wantErr:     true,
		},
		{
			name:        "empty string",
			principalID: "",
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDN, gotScope, err := p.getDNAndScopeFromPrincipalID(tt.principalID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantDN, gotDN)
			assert.Equal(t, tt.wantScope, gotScope)
		})
	}
}
