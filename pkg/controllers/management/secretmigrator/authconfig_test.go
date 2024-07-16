package secretmigrator

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
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
)

const (
	testPassword            = "testpass1234"
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

func (c *unstructuredConfig) UnstructuredContent() map[string]interface{} {
	return c.values
}

func (c *unstructuredConfig) SetUnstructuredContent(m map[string]interface{}) {
	c.values = m
}

func (c *unstructuredConfig) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

// EachListItemWithAlloc implements runtime.Unstructured.
func (*unstructuredConfig) EachListItemWithAlloc(func(runtime.Object) error) error {
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
		wantConditions            []condition.Cond
		wantSecretRef             string
	}{
		{
			name:                   "test migrating Shibboleth configuration with openLDAP",
			expectedSecretName:     fmt.Sprintf("shibbolethconfig-%s", strings.ToLower(serviceAccountPasswordFieldName)),
			authConfig:             newTestShibbolethConfig(),
			unstructuredAuthConfig: getUnstructuredShibbolethConfig(withOpenLDAP),
			expectedError:          false,
			openLDAPEnabled:        true,
			wantConditions: []condition.Cond{
				apimgmtv3.AuthConfigConditionSecretsMigrated,
				apimgmtv3.AuthConfigConditionShibbolethSecretFixed,
			},
			wantSecretRef: fmt.Sprintf("cattle-global-data:shibbolethconfig-%s", strings.ToLower(serviceAccountPasswordFieldName)),
		},
		{
			name: "test migrating existing Shibboleth config",
			authConfig: newTestShibbolethConfig(func(ac *apimgmtv3.AuthConfig) {
				ac.Status = apimgmtv3.AuthConfigStatus{
					Conditions: []apimgmtv3.AuthConfigConditions{
						apimgmtv3.AuthConfigConditions{
							Type:           apimgmtv3.AuthConfigConditionSecretsMigrated,
							Status:         "True",
							LastUpdateTime: "2024-05-13T15:20:34+01:00",
						},
					},
				}
			}),
			unstructuredAuthConfig: getUnstructuredShibbolethConfig(),
			wantConditions:         []condition.Cond{},
		},
		{
			name:                   "test migrating Shibboleth configuration without OpenLDAP",
			authConfig:             newTestShibbolethConfig(),
			unstructuredAuthConfig: getUnstructuredShibbolethConfig(),
			expectedError:          false,
			openLDAPEnabled:        false,
			wantConditions:         []condition.Cond{apimgmtv3.AuthConfigConditionSecretsMigrated},
			wantSecretRef:          fmt.Sprintf("cattle-global-data:shibbolethconfig-%s", strings.ToLower(serviceAccountPasswordFieldName)),
		},
		{
			name:            "test migrating non Shibboleth configuration",
			authConfig:      getMockNonShibbolethConfig(),
			expectedError:   false,
			openLDAPEnabled: false,
			wantConditions:  []condition.Cond{},
		},
		{
			name: "test migrating Shibboleth with incorrect secret name",
			authConfig: newTestShibbolethConfig(func(ac *apimgmtv3.AuthConfig) {
				ac.Status = apimgmtv3.AuthConfigStatus{
					Conditions: []apimgmtv3.AuthConfigConditions{
						apimgmtv3.AuthConfigConditions{
							Type:           apimgmtv3.AuthConfigConditionSecretsMigrated,
							Status:         "True",
							LastUpdateTime: "2024-05-13T15:20:34+01:00",
						},
					},
				}
			}),
			unstructuredAuthConfig: getUnstructuredShibbolethConfig(func(s map[string]any) {
				s["openLdapConfig"] = map[string]any{
					// This is the incorrect secret name from SURE-7772
					"serviceAccountPassword": "cattle-global-data:shibbolethconfig-serviceAccountPassword",
				}
			}),
			expectedError:   false,
			openLDAPEnabled: true,
			wantConditions: []condition.Cond{
				apimgmtv3.AuthConfigConditionShibbolethSecretFixed,
			},
			wantSecretRef: fmt.Sprintf("cattle-global-data:shibbolethconfig-%s", strings.ToLower(serviceAccountPasswordFieldName)),
		},
		{
			name: "test migrating Shibboleth with different secret name",
			authConfig: newTestShibbolethConfig(func(ac *apimgmtv3.AuthConfig) {
				ac.Status = apimgmtv3.AuthConfigStatus{
					Conditions: []apimgmtv3.AuthConfigConditions{
						apimgmtv3.AuthConfigConditions{
							Type:           apimgmtv3.AuthConfigConditionSecretsMigrated,
							Status:         "True",
							LastUpdateTime: "2024-05-13T15:20:34+01:00",
						},
					},
				}
			}),
			unstructuredAuthConfig: getUnstructuredShibbolethConfig(func(s map[string]any) {
				s["openLdapConfig"] = map[string]any{
					// This is perhaps a user-configured name.
					"serviceAccountPassword": "cattle-global-data:testing-Password",
				}
			}),
			expectedError:   false,
			openLDAPEnabled: true,
			wantConditions: []condition.Cond{
				apimgmtv3.AuthConfigConditionShibbolethSecretFixed,
			},
			wantSecretRef: fmt.Sprintf("cattle-global-data:testing-Password"),
		},
		{
			name:               "test migrating Shibboleth without migrated secret",
			expectedSecretName: fmt.Sprintf("shibbolethconfig-%s", strings.ToLower(serviceAccountPasswordFieldName)),
			authConfig:         newTestShibbolethConfig(),
			unstructuredAuthConfig: getUnstructuredShibbolethConfig(func(s map[string]any) {
				s["openLdapConfig"] = map[string]any{
					"serviceAccountPassword": testPassword,
				}
			}),
			expectedError:   false,
			openLDAPEnabled: true,
			wantConditions: []condition.Cond{
				apimgmtv3.AuthConfigConditionSecretsMigrated,
				apimgmtv3.AuthConfigConditionShibbolethSecretFixed,
			},
			wantSecretRef: fmt.Sprintf("cattle-global-data:shibbolethconfig-%s", strings.ToLower(serviceAccountPasswordFieldName)),
		},
	}

	conditionTypes := func(s apimgmtv3.AuthConfigStatus) []condition.Cond {
		var result []condition.Cond
		for _, c := range s.Conditions {
			result = append(result, c.Type)
		}

		return result
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHandler(
				tt.unstructuredAuthConfig,
				func(secret *corev1.Secret) (*corev1.Secret, error) {
					if tt.expectedErrorCreateSecret {
						return nil, errorCreateSecret
					}

					assert.Equal(t, tt.expectedSecretName, secret.Name, "secret name did not match")
					assert.Equal(t, namespace.GlobalNamespace, secret.Namespace)
					assert.Equal(t, testPassword, secret.StringData[strings.ToLower(serviceAccountPasswordFieldName)])

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

				shibbConfig := config.(*apimgmtv3.ShibbolethConfig)

				assert.NotNil(t, shibbConfig)

				assert.Equal(t, tt.wantConditions, conditionTypes(shibbConfig.Status))
				assert.Equal(t, tt.authConfig.ObjectMeta, shibbConfig.SamlConfig.ObjectMeta)
				assert.Equal(t, tt.authConfig.TypeMeta, shibbConfig.SamlConfig.TypeMeta)
				assert.Equal(t, tt.wantSecretRef, shibbConfig.OpenLdapConfig.ServiceAccountPassword)

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, config)
		})
	}
}

func TestOKTAAuthConfigMigration(t *testing.T) {
	timeStamp, _ := time.Parse(time.RFC3339, testCreationStampString)
	testcases := []struct {
		name                   string
		unstructuredAuthConfig map[string]any
		authConfig             apimgmtv3.AuthConfig
		expectedLdapConfig     apimgmtv3.LdapFields
		wantStringData         map[string]string
		// wantMigration is true when we expect the migration to execute
		wantMigration bool
	}{
		{
			name:                   "test migrating OKTA configuration with openLDAP",
			unstructuredAuthConfig: getUnstructuredOKTA(withOpenLDAP),
			authConfig: apimgmtv3.AuthConfig{
				Type:    "oktaConfig",
				Enabled: true,
				ObjectMeta: metav1.ObjectMeta{
					Name:              "okta",
					CreationTimestamp: metav1.NewTime(timeStamp),
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "AuthConfig",
					APIVersion: "management.cattle.io/v3",
				},
			},
			expectedLdapConfig: apimgmtv3.LdapFields{
				ServiceAccountPassword: "cattle-global-data:oktaconfig-serviceaccountpassword",
			},
			wantStringData: map[string]string{
				"serviceaccountpassword": "testpass1234",
			},
			wantMigration: true,
		},
		{
			name:                   "test migrating with existing migration",
			unstructuredAuthConfig: getUnstructuredOKTA(withOpenLDAP),
			wantMigration:          true,
			authConfig: apimgmtv3.AuthConfig{
				Type:    "oktaConfig",
				Enabled: true,
				ObjectMeta: metav1.ObjectMeta{
					Name:              "okta",
					CreationTimestamp: metav1.NewTime(timeStamp),
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "AuthConfig",
					APIVersion: "management.cattle.io/v3",
				},
				Status: apimgmtv3.AuthConfigStatus{
					Conditions: []apimgmtv3.AuthConfigConditions{
						apimgmtv3.AuthConfigConditions{
							Type:               apimgmtv3.AuthConfigConditionSecretsMigrated,
							Status:             "True",
							LastUpdateTime:     "2024-05-13T15:20:34+01:00",
							LastTransitionTime: "",
							Reason:             "",
							Message:            "",
						},
					},
				},
			},
			expectedLdapConfig: apimgmtv3.LdapFields{
				ServiceAccountPassword: "cattle-global-data:oktaconfig-serviceaccountpassword",
			},
			wantStringData: map[string]string{
				"serviceaccountpassword": "testpass1234",
			},
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			h := newFakeHandler(
				tt.unstructuredAuthConfig,
				func(secret *corev1.Secret) (*corev1.Secret, error) {
					assert.Equal(t, "oktaconfig-serviceaccountpassword", secret.Name)
					assert.Equal(t, namespace.GlobalNamespace, secret.Namespace)
					assert.Equal(t, tt.wantStringData, secret.StringData)

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

			assert.NotNil(t, config)
			assert.NoError(t, err)

			oktaConfig, ok := config.(*apimgmtv3.OKTAConfig)

			assert.Equal(t, tt.wantMigration, ok)
			if !tt.wantMigration {
				return
			}
			assert.NotNil(t, oktaConfig)

			assert.NotEmpty(t, oktaConfig.Status.Conditions)
			assert.NotNil(t, oktaConfig.Status.Conditions[0])
			assert.Equal(t, apimgmtv3.AuthConfigOKTAPasswordMigrated, oktaConfig.Status.Conditions[0].Type)
			assert.Equal(t, tt.expectedLdapConfig, oktaConfig.OpenLdapConfig)
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

func newTestShibbolethConfig(opts ...func(*apimgmtv3.AuthConfig)) apimgmtv3.AuthConfig {
	timeStamp, _ := time.Parse(time.RFC3339, testCreationStampString)
	ac := apimgmtv3.AuthConfig{
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

	for _, opt := range opts {
		opt(&ac)
	}

	return ac
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

func withOpenLDAP(s map[string]any) {
	s["openLdapConfig"] = map[string]any{
		"serviceAccountPassword": testPassword,
	}
}

func getUnstructuredShibbolethConfig(opts ...func(map[string]any)) map[string]any {
	timeStamp, _ := time.Parse(time.RFC3339, testCreationStampString)
	createdTime := metav1.NewTime(timeStamp)

	raw := map[string]any{
		"metadata": map[string]any{
			"name":              saml.ShibbolethName,
			"creationtimestamp": createdTime,
		},
		"kind":       "AuthConfig",
		"apiVersion": "management.cattle.io/v3",
		"type":       client.ShibbolethConfigType,
		"enabled":    true,
	}

	for _, o := range opts {
		o(raw)
	}

	return raw
}

func getUnstructuredOKTA(opts ...func(map[string]any)) map[string]any {
	timeStamp, _ := time.Parse(time.RFC3339, testCreationStampString)
	createdTime := metav1.NewTime(timeStamp)

	raw := map[string]any{
		"metadata": map[string]any{
			"name":              "okta",
			"creationtimestamp": createdTime,
		},
		"kind":                   "AuthConfig",
		"apiVersion":             "management.cattle.io/v3",
		"type":                   client.OKTAConfigType,
		"enabled":                true,
		"serviceAccountPassword": testPassword,
	}

	for _, o := range opts {
		o(raw)
	}

	return raw
}

type mockAuthConfigClient struct {
	config map[string]any
}

func newMockAuthConfigClient(authConfig map[string]any) authConfigsClient {
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
