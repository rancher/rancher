package ldap

import (
	"context"
	"crypto/x509"
	"reflect"
	"testing"

	"github.com/rancher/norman/objectclient"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
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
		input interface{}
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
		ctx                   context.Context
		secrets               wcorev1.SecretController
		userMGR               user.Manager
		tokenMGR              *tokens.Manager
		certs                 string
		caPool                *x509.CertPool
		providerName          string
		testAndApplyInputType string
		userScope             string
		groupScope            string
	}
	tests := []struct {
		name                 string
		objectMap            map[string]interface{}
		fields               fields
		wantStoredLdapConfig *v3.LdapConfig
		wantCaPool           *x509.CertPool
		wantErr              bool
	}{
		{
			name: "get LDAP config object",
			fields: fields{
				caPool: x509.NewCertPool(),
				certs:  DummyCerts,
			},
			wantStoredLdapConfig: &v3.LdapConfig{
				LdapFields: v3.LdapFields{
					Certificate: DummyCerts,
				},
			},
			objectMap: map[string]interface{}{
				"Certificate": DummyCerts,
			},
			wantCaPool: x509.NewCertPool(),
			wantErr:    false,
		},
		{
			name: "ldap config is nil",
			fields: fields{
				providerName: "okta",
			},
			objectMap: map[string]interface{}{
				"openLdapConfig": nil,
			},
			wantErr: true,
		},
		{
			name: "ldap config not found",
			fields: fields{
				providerName: "okta",
			},
			objectMap: map[string]interface{}{},
			wantErr:   true,
		},
		{
			name: "no servers found",
			fields: fields{
				providerName: "okta",
			},
			objectMap: map[string]interface{}{
				"openLdapConfig": map[string]interface{}{
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
				caPool:       x509.NewCertPool(),
				certs:        DummyCerts,
			},
			objectMap: map[string]interface{}{
				"openLdapConfig": map[string]interface{}{
					"Certificate": DummyCerts,
					"servers":     []string{"server1"},
				},
			},
			wantStoredLdapConfig: &v3.LdapConfig{
				LdapFields: v3.LdapFields{
					Servers:     []string{"server1"},
					Certificate: DummyCerts,
				},
			},
			wantCaPool: x509.NewCertPool(),
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mockGenericClient{ObjectMap: tt.objectMap}
			p := &ldapProvider{
				ctx:                   tt.fields.ctx,
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
			gotStoredLdapConfig, gotCaPool, err := p.getLDAPConfig(m)
			if (err != nil) != tt.wantErr {
				t.Errorf("ldapProvider.getLDAPConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotStoredLdapConfig, tt.wantStoredLdapConfig) {
				t.Errorf("ldapProvider.getLDAPConfig() got ldapConfig = %v, want %v", gotStoredLdapConfig, tt.wantStoredLdapConfig)
			}
			if !reflect.DeepEqual(gotCaPool, tt.wantCaPool) {
				t.Errorf("ldapProvider.getLDAPConfig() got caPool = %v, want %v", gotCaPool, tt.wantCaPool)
			}
		})
	}
}

type mockGenericClient struct {
	ObjectMap map[string]interface{}
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
