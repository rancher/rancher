package secretmigrator

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/objectclient"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/core/v1/fakes"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	mockPass                = "testpass1234"
	testCreationStampString = "2023-05-15T19:28:22Z"
)

func TestSetUnstructuredStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  unstructuredConfig
		output unstructuredConfig
	}{
		{
			name: "config with no status",
			input: unstructuredConfig{
				values: map[string]any{
					"foo": "bar",
				},
			},
			output: unstructuredConfig{
				values: map[string]any{
					"foo": "bar",
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"status": "True",
								"type":   "SecretsMigrated",
							},
						},
					},
				},
			},
		},
		{
			name: "config has a status with no conditions",
			input: unstructuredConfig{
				values: map[string]any{
					"foo":    "bar",
					"status": map[string]any{},
				},
			},
			output: unstructuredConfig{
				values: map[string]any{
					"foo": "bar",
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"status": "True",
								"type":   "SecretsMigrated",
							},
						},
					},
				},
			},
		},
		{
			name: "config has a status with no matching conditions",
			input: unstructuredConfig{
				values: map[string]any{
					"foo": "bar",
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"status": "Foo",
								"type":   "Something",
							},
						},
					},
				},
			},
			output: unstructuredConfig{
				values: map[string]any{
					"foo": "bar",
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"status": "Foo",
								"type":   "Something",
							},
							map[string]any{
								"status": "True",
								"type":   "SecretsMigrated",
							},
						},
					},
				},
			},
		},
		{
			name: "config has a status with matching condition",
			input: unstructuredConfig{
				values: map[string]any{
					"foo": "bar",
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"status": "Unknown",
								"type":   "SecretsMigrated",
							},
						},
					},
				},
			},
			output: unstructuredConfig{
				values: map[string]any{
					"foo": "bar",
					"status": map[string]any{
						"conditions": []any{
							map[string]any{
								"status": "True",
								"type":   "SecretsMigrated",
							},
						},
					},
				},
			},
		},
	}

	const cond = apimgmtv3.AuthConfigConditionSecretsMigrated
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := setUnstructuredStatus(&test.input, cond, "True")
			if err != nil {
				t.Fatalf("got an unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, &test.output) {
				t.Errorf("expected %+v, got %+v", test.output, got)
			}
		})
	}

}

type unstructuredConfig struct {
	values map[string]any
}

// EachListItemWithAlloc implements runtime.Unstructured.
func (*unstructuredConfig) EachListItemWithAlloc(func(runtime.Object) error) error {
	panic("implement me")
}

func (c *unstructuredConfig) UnstructuredContent() map[string]interface{} {
	return c.values
}

func (c *unstructuredConfig) SetUnstructuredContent(m map[string]interface{}) {
	c.values = m
}

func (c *unstructuredConfig) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

func (c *unstructuredConfig) DeepCopyObject() runtime.Object {
	panic("implement me")
}

func (c *unstructuredConfig) NewEmptyInstance() runtime.Unstructured {
	panic("implement me")
}

func (c *unstructuredConfig) IsList() bool {
	panic("implement me")
}

func (c *unstructuredConfig) EachListItem(f func(runtime.Object) error) error {
	panic("implement me")
}

func TestShibbolethAuthConfigMigration(t *testing.T) {
	errorCreateSecret := fmt.Errorf("failed to create secret")

	testcases := []struct {
		name                      string
		unstructuredAuthConfig    map[string]any
		authConfig                apimgmtv3.AuthConfig
		expectedSecretName        string
		expectedError             bool
		openLDAPEnabled           bool
		expectedErrorCreateSecret bool
	}{
		{
			name:                   "test migrating Shibboleth configuration with openLDAP",
			expectedSecretName:     fmt.Sprintf("shibbolethconfig-%s", strings.ToLower(serviceAccountPasswordFieldName)),
			authConfig:             getMockShibbolethConfig(),
			unstructuredAuthConfig: getMockShibbolethWithOpenLDAP(),
			expectedError:          false,
			openLDAPEnabled:        true,
		},
		{
			name:                   "test migrating Shibboleth configuration without OpenLDAP",
			authConfig:             getMockShibbolethConfig(),
			unstructuredAuthConfig: getMockShibbolethWithoutOpenLDAP(),
			expectedError:          false,
			openLDAPEnabled:        false,
		},
		{
			name:            "test migrating non Shibboleth configuration",
			authConfig:      getMockNonShibbolethConfig(),
			expectedError:   false,
			openLDAPEnabled: false,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHandler(
				tt.unstructuredAuthConfig,
				func(secret *corev1.Secret) (*corev1.Secret, error) {
					if tt.expectedErrorCreateSecret {
						return nil, errorCreateSecret
					}

					assert.Equal(t, tt.expectedSecretName, secret.Name)
					assert.Equal(t, namespace.GlobalNamespace, secret.Namespace)
					assert.Equal(t, mockPass, secret.StringData[strings.ToLower(serviceAccountPasswordFieldName)])

					return secret, nil
				},
				func(secret *corev1.Secret) (*corev1.Secret, error) {
					return nil, nil
				},
				func(namespace string, name string) (*corev1.Secret, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
				},
			)

			config, err := h.syncAuthConfig("test", &tt.authConfig)

			if tt.openLDAPEnabled {
				if tt.expectedError {
					assert.Error(t, err)
					assert.Nil(t, config)
					return
				}

				if tt.expectedErrorCreateSecret {
					assert.Error(t, err)
					assert.True(t, errors.Is(err, errorCreateSecret))
					assert.Nil(t, config)
					return
				}

				assert.NotNil(t, config)
				assert.NoError(t, err)

				shibbConfig, ok := config.(*apimgmtv3.ShibbolethConfig)

				assert.True(t, ok)
				assert.NotNil(t, shibbConfig)

				assert.NotEmpty(t, shibbConfig.Status.Conditions)
				assert.NotNil(t, shibbConfig.Status.Conditions[0])
				assert.Equal(t, apimgmtv3.AuthConfigConditionSecretsMigrated, shibbConfig.Status.Conditions[0].Type)

				assert.Equal(t, tt.authConfig.ObjectMeta, shibbConfig.SamlConfig.ObjectMeta)
				assert.Equal(t, tt.authConfig.TypeMeta, shibbConfig.SamlConfig.TypeMeta)

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, config)
		})
	}
}

func newFakeHandler(
	authConfig map[string]any,
	secretCreateFunc func(*corev1.Secret) (*corev1.Secret, error),
	secretUpdateFunc func(*corev1.Secret) (*corev1.Secret, error),
	secretGetFunc func(string, string) (*corev1.Secret, error),
) *handler {
	secretInterfaceMock := corefakes.SecretInterfaceMock{
		CreateFunc: secretCreateFunc,
		UpdateFunc: secretUpdateFunc,
	}

	secretListerMock := corefakes.SecretListerMock{
		GetFunc: secretGetFunc,
	}

	h := &handler{
		migrator:         NewMigrator(&secretListerMock, &secretInterfaceMock),
		authConfigs:      newMockAuthConfigClient(authConfig),
		authConfigLister: &fakes.AuthConfigListerMock{},
	}
	return h
}

func getMockShibbolethConfig() apimgmtv3.AuthConfig {
	timeStamp, _ := time.Parse(time.RFC3339, testCreationStampString)
	return apimgmtv3.AuthConfig{
		Type:    "shibbolethConfig",
		Enabled: true,
		ObjectMeta: metav1.ObjectMeta{
			Name:              saml.ShibbolethName,
			CreationTimestamp: metav1.NewTime(timeStamp),
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "AuthConfig",
			APIVersion: "management.cattle.io/v3",
		},
	}
}

func getMockNonShibbolethConfig() apimgmtv3.AuthConfig {
	return apimgmtv3.AuthConfig{
		Type:    "NOTshibbolethConfig",
		Enabled: true,
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "AuthConfig",
			APIVersion: "management.cattle.io/v3",
		},
	}
}

func getMockShibbolethWithoutOpenLDAP() map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"name": saml.ShibbolethName,
		},
		"kind":       "AuthConfig",
		"apiVersion": "management.cattle.io/v3",
		"type":       "shibbolethConfig",
		"enabled":    true,
	}
}

func getMockShibbolethWithOpenLDAP() map[string]any {
	timeStamp, _ := time.Parse(time.RFC3339, testCreationStampString)
	createdTime := metav1.NewTime(timeStamp)
	return map[string]any{
		"metadata": map[string]any{
			"name":              saml.ShibbolethName,
			"creationtimestamp": createdTime,
		},
		"kind":       "AuthConfig",
		"apiVersion": "management.cattle.io/v3",
		"type":       "shibbolethConfig",
		"enabled":    true,
		"openLdapConfig": map[string]any{
			"serviceAccountPassword": mockPass,
		},
	}
}

type mockAuthConfigClient struct {
	config map[string]any
}

func newMockAuthConfigClient(authConfig map[string]any) objectclient.GenericClient {
	return mockAuthConfigClient{config: authConfig}
}

func (m mockAuthConfigClient) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	o := unstructured.Unstructured{}
	o.SetUnstructuredContent(m.config)
	return &o, nil
}

func (m mockAuthConfigClient) Update(name string, o runtime.Object) (runtime.Object, error) {
	return o, nil
}

func (m mockAuthConfigClient) UnstructuredClient() objectclient.GenericClient {
	panic("implement me")
}

func (m mockAuthConfigClient) GroupVersionKind() schema.GroupVersionKind {
	panic("implement me")
}

func (m mockAuthConfigClient) Create(o runtime.Object) (runtime.Object, error) {
	panic("implement me")
}

func (m mockAuthConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
	panic("implement me")
}

func (m mockAuthConfigClient) UpdateStatus(name string, o runtime.Object) (runtime.Object, error) {
	panic("implement me")
}

func (m mockAuthConfigClient) DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error {
	panic("implement me")
}

func (m mockAuthConfigClient) Delete(name string, opts *metav1.DeleteOptions) error {
	panic("implement me")
}

func (m mockAuthConfigClient) List(opts metav1.ListOptions) (runtime.Object, error) {
	panic("implement me")
}

func (m mockAuthConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	panic("implement me")
}

func (m mockAuthConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	panic("implement me")
}

func (m mockAuthConfigClient) DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	panic("implement me")
}

func (m mockAuthConfigClient) Patch(name string, o runtime.Object, patchType types.PatchType, data []byte, subresources ...string) (runtime.Object, error) {
	panic("implement me")
}

func (m mockAuthConfigClient) ObjectFactory() objectclient.ObjectFactory {
	panic("implement me")
}

func (m mockAuthConfigClient) ObjectClient() *objectclient.ObjectClient {
	panic("implement me")
}
