package ldap

import (
	"context"
	"crypto/x509"
	"reflect"
	"testing"

	"github.com/rancher/norman/objectclient"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/tokens"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/user"
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

func Test_getBasicLogin(t *testing.T) {
	type args struct {
		input interface{}
	}
	tests := []struct {
		name      string
		args      args
		wantLogin *v32.BasicLogin
		wantErr   bool
	}{
		{
			name: "good input credentials",
			args: args{
				input: &v32.BasicLogin{
					Username: DummyUsername,
					Password: DummyPassword,
				},
			},
			wantLogin: &v32.BasicLogin{
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
			wantLogin: &v32.BasicLogin{},
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

func Test_ldapProvider_getLDAPConfig(t *testing.T) {
	type fields struct {
		ctx                   context.Context
		authConfigs           v3.AuthConfigInterface
		secrets               corev1.SecretInterface
		userMGR               user.Manager
		tokenMGR              *tokens.Manager
		certs                 string
		caPool                *x509.CertPool
		providerName          string
		testAndApplyInputType string
		userScope             string
		groupScope            string
		mockGenericClient     mockGenericClient
	}
	tests := []struct {
		name                 string
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
				LdapFields: v32.LdapFields{
					Certificate: DummyCerts,
				},
			},
			wantCaPool: x509.NewCertPool(),
			wantErr:    false,
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
			gotStoredLdapConfig, gotCaPool, err := p.getLDAPConfig(tt.fields.mockGenericClient)
			if (err != nil) != tt.wantErr {
				t.Errorf("ldapProvider.getLDAPConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotStoredLdapConfig, tt.wantStoredLdapConfig) {
				t.Errorf("ldapProvider.getLDAPConfig() got = %v, want %v", gotStoredLdapConfig, tt.wantStoredLdapConfig)
			}
			if !reflect.DeepEqual(gotCaPool, tt.wantCaPool) {
				t.Errorf("ldapProvider.getLDAPConfig() got1 = %v, want %v", gotCaPool, tt.wantCaPool)
			}
		})
	}
}

type mockGenericClient struct{}

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
		Object: map[string]interface{}{
			"Certificate": DummyCerts,
		},
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
