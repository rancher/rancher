package nodedriver

import (
	"fmt"
	"testing"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestGetCredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		annotations      map[string]string
		expectedPublic   []string
		expectedPrivate  []string
		expectedPassword sets.Set[string]
		expectedOptional sets.Set[string]
		expectedDefaults map[string]string
	}{
		{
			name:             "nil annotations return empty metadata",
			annotations:      nil,
			expectedPublic:   nil,
			expectedPrivate:  nil,
			expectedPassword: sets.New[string](),
			expectedOptional: sets.New[string](),
			expectedDefaults: map[string]string{},
		},
		{
			name: "annotation fields are normalized into sorted sets",
			annotations: map[string]string{
				"publicCredentialFields":   "username,accessKey,,endpoint,accessKey",
				"privateCredentialFields":  ",password,secretKey,password",
				"passwordFields":           "password,,token,password",
				"optionalCredentialFields": "endpoint,,password,endpoint",
				"defaults":                 "endpoint:example.com,region:us-west-2",
			},
			expectedPublic:   []string{"accessKey", "endpoint", "username"},
			expectedPrivate:  []string{"password", "secretKey"},
			expectedPassword: sets.New[string]("password", "token"),
			expectedOptional: sets.New[string]("endpoint", "password"),
			expectedDefaults: map[string]string{
				"endpoint": "example.com",
				"region":   "us-west-2",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := getCredFields(tt.annotations)

			assert.Equal(t, tt.expectedPublic, result.publicList())
			assert.Equal(t, tt.expectedPrivate, result.privateList())
			assert.Equal(t, tt.expectedPassword, result.password)
			assert.Equal(t, tt.expectedOptional, result.optional)
			assert.Equal(t, tt.expectedDefaults, result.defaults)
			assert.False(t, result.public.Has(""))
			assert.False(t, result.private.Has(""))
			assert.False(t, result.password.Has(""))
			assert.False(t, result.optional.Has(""))
		})
	}
}

// fakeSchemaLister is a minimal DynamicSchemaLister backed by a map, keyed by
// schema name. A non-nil getErr is returned from Get to simulate lister
// failures.
type fakeSchemaLister struct {
	schemas map[string]*v32.DynamicSchema
	getErr  error
}

func (f fakeSchemaLister) List(namespace string, selector labels.Selector) ([]*v32.DynamicSchema, error) {
	var result []*v32.DynamicSchema
	for _, schema := range f.schemas {
		result = append(result, schema)
	}
	return result, nil
}

func (f fakeSchemaLister) Get(namespace, name string) (*v32.DynamicSchema, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	schema, ok := f.schemas[name]
	if !ok {
		return nil, errors.NewNotFound(schemaGroupResource, name)
	}
	return schema, nil
}

var schemaGroupResource = schema.GroupResource{Group: "management.cattle.io", Resource: "dynamicschemas"}

func TestResolveCredFieldMetadata(t *testing.T) {
	t.Parallel()

	annotations := map[string]string{
		"publicCredentialFields":  "accessKey",
		"privateCredentialFields": "secretKey",
		"passwordFields":          "secretKey",
	}

	tests := []struct {
		name            string
		schemas         map[string]*v32.DynamicSchema
		getErr          error
		expectedPublic  sets.Set[string]
		expectedPrivate sets.Set[string]
		expectErr       bool
	}{
		{
			name: "schema spec takes precedence over annotations",
			schemas: map[string]*v32.DynamicSchema{
				"amazonec2credentialconfig": {
					Spec: v32.DynamicSchemaSpec{
						PublicFields:  []string{"foo", "bar"},
						PrivateFields: []string{"baz"},
					},
				},
			},
			expectedPublic:  sets.New("bar", "foo"),
			expectedPrivate: sets.New("baz"),
		},
		{
			name:            "falls back to annotations when schema does not exist",
			schemas:         map[string]*v32.DynamicSchema{},
			expectedPublic:  sets.New("accessKey"),
			expectedPrivate: sets.New("secretKey"),
		},
		{
			name: "falls back to annotations when spec fields are empty",
			schemas: map[string]*v32.DynamicSchema{
				"amazonec2credentialconfig": {
					Spec: v32.DynamicSchemaSpec{},
				},
			},
			expectedPublic:  sets.New("accessKey"),
			expectedPrivate: sets.New("secretKey"),
		},
		{
			name: "spec public fields override annotations, empty private spec falls back to annotations",
			schemas: map[string]*v32.DynamicSchema{
				"amazonec2credentialconfig": {
					Spec: v32.DynamicSchemaSpec{
						PublicFields: []string{"foo"},
					},
				},
			},
			expectedPublic:  sets.New("foo"),
			expectedPrivate: sets.New("secretKey"),
		},
		{
			name:      "lister errors other than not found are propagated",
			getErr:    errors.NewInternalError(fmt.Errorf("cache read failure")),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lifecycle := &Lifecycle{schemaLister: fakeSchemaLister{schemas: tt.schemas, getErr: tt.getErr}}
			obj := &v32.NodeDriver{
				ObjectMeta: metav1.ObjectMeta{Name: "amazonec2"},
				Spec:       v32.NodeDriverSpec{DisplayName: "amazonec2"},
			}
			obj.Annotations = annotations

			result, err := lifecycle.resolveCredFieldMetadata(obj)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPublic, result.public)
			assert.Equal(t, tt.expectedPrivate, result.private)
			// non-public/private metadata is always annotation-derived
			assert.Equal(t, sets.New("secretKey"), result.password)
		})
	}
}

func TestCredFieldAnnotationChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		annotations   map[string]string
		publicFields  []string
		privateFields []string
		expected      map[string]string
	}{
		{
			name: "no changes when annotations match as sets regardless of order and duplicates",
			annotations: map[string]string{
				"publicCredentialFields":  "username,accessKey,,endpoint,accessKey",
				"privateCredentialFields": "secretKey,password",
			},
			publicFields:  []string{"accessKey", "endpoint", "username"},
			privateFields: []string{"password", "secretKey"},
			expected:      nil,
		},
		{
			name: "public fields diverge from annotation",
			annotations: map[string]string{
				"publicCredentialFields":  "accessKey",
				"privateCredentialFields": "secretKey",
			},
			publicFields:  []string{"extraField", "accessKey"},
			privateFields: []string{"secretKey"},
			expected: map[string]string{
				"publicCredentialFields": "accessKey,extraField",
			},
		},
		{
			name: "private fields diverge from annotation",
			annotations: map[string]string{
				"publicCredentialFields":  "accessKey",
				"privateCredentialFields": "secretKey",
			},
			publicFields:  []string{"accessKey"},
			privateFields: []string{"secretKey", "token"},
			expected: map[string]string{
				"privateCredentialFields": "secretKey,token",
			},
		},
		{
			name: "missing annotations are treated as divergent",
			annotations: map[string]string{
				"privateCredentialFields": "secretKey",
			},
			publicFields:  []string{"accessKey"},
			privateFields: []string{"secretKey"},
			expected: map[string]string{
				"publicCredentialFields": "accessKey",
			},
		},
		{
			name: "empty resolved lists never clear annotations",
			annotations: map[string]string{
				"publicCredentialFields":  "accessKey",
				"privateCredentialFields": "secretKey",
			},
			publicFields:  nil,
			privateFields: nil,
			expected:      nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.expected, credFieldAnnotationChanges(tt.annotations, tt.publicFields, tt.privateFields))
		})
	}
}

func TestSyntheticCredentialFields(t *testing.T) {
	t.Parallel()

	synthetic, ok := syntheticCredentialFields["amazonec2"]["defaultRegion"]
	assert.True(t, ok, "amazonec2 should define a synthetic defaultRegion field")
	assert.Equal(t, "string", synthetic.field.Type)
	assert.Equal(t, "AWS Default Region", synthetic.field.Description)
	assert.True(t, synthetic.field.DynamicField)
	assert.True(t, synthetic.field.Create)
	assert.True(t, synthetic.field.Update)
	assert.True(t, synthetic.public, "defaultRegion should be classified as public")
}

// TestCreateCredSchemaSyncsAnnotations verifies the end-to-end annotation sync
// flow in createCredSchema: when the resolved public/private field lists
// diverge from the node driver annotations, the node driver is updated with
// the authoritative values. Synthetic (virtual) fields such as the amazonec2
// defaultRegion are applied to the schema spec but excluded from the
// annotations, and update failures are propagated.
func TestCreateCredSchemaSyncsAnnotations(t *testing.T) {
	t.Parallel()

	syntheticDefaultRegion := syntheticCredentialFields["amazonec2"]["defaultRegion"].field

	// newTestLifecycle returns a lifecycle whose credential config schema
	// already matches the fields that createCredSchema would produce for
	// the given driver-declared public fields (including the merged
	// synthetic defaultRegion field), so the schema itself does not need an
	// update and the annotation sync can be observed in isolation.
	newTestLifecycle := func(driverClient *fakes.NodeDriverInterfaceMock, driverPublicFields []string) (*Lifecycle, *fakes.DynamicSchemaInterfaceMock) {
		schemaClient := &fakes.DynamicSchemaInterfaceMock{
			UpdateFunc: func(in1 *v32.DynamicSchema) (*v32.DynamicSchema, error) {
				return in1, nil
			},
		}
		resourceFields := map[string]v32.Field{
			"accessKey":     {Type: "string"},
			"defaultRegion": syntheticDefaultRegion,
		}
		publicFields := append([]string{"defaultRegion"}, driverPublicFields...)
		for _, name := range driverPublicFields {
			if _, ok := resourceFields[name]; !ok {
				resourceFields[name] = v32.Field{Type: "string"}
			}
		}
		credSchema := &v32.DynamicSchema{
			Spec: v32.DynamicSchemaSpec{
				ResourceFields: resourceFields,
				PublicFields:   sortedSetList(sets.New(publicFields...)),
				PrivateFields:  []string{"secretKey"},
			},
		}
		lifecycle := &Lifecycle{
			schemaLister: fakeSchemaLister{schemas: map[string]*v32.DynamicSchema{
				"amazonec2credentialconfig": credSchema,
			}},
			schemaClient:     schemaClient,
			nodeDriverClient: driverClient,
		}
		return lifecycle, schemaClient
	}

	newDriver := func(annotations map[string]string) *v32.NodeDriver {
		return &v32.NodeDriver{
			ObjectMeta: metav1.ObjectMeta{Name: "amazonec2", Annotations: annotations},
			Spec:       v32.NodeDriverSpec{DisplayName: "amazonec2"},
		}
	}

	credFields := func(names ...string) map[string]v32.Field {
		fields := map[string]v32.Field{}
		for _, name := range names {
			fields[name] = v32.Field{Type: "string"}
		}
		return fields
	}

	t.Run("divergent non-synthetic spec values are synced onto the node driver annotations", func(t *testing.T) {
		t.Parallel()

		var updated *v32.NodeDriver
		driverClient := &fakes.NodeDriverInterfaceMock{
			UpdateFunc: func(in1 *v32.NodeDriver) (*v32.NodeDriver, error) {
				updated = in1
				return in1, nil
			},
		}
		lifecycle, schemaClient := newTestLifecycle(driverClient, []string{"accessKey", "newField"})
		obj := newDriver(map[string]string{
			"publicCredentialFields":  "accessKey",
			"privateCredentialFields": "secretKey",
		})

		result, err := lifecycle.createCredSchema(obj, credFields("accessKey", "newField"), []string{"accessKey", "newField"}, []string{"secretKey"})

		assert.NoError(t, err)
		assert.NotNil(t, updated)
		assert.Same(t, updated, result)
		assert.Equal(t, "accessKey,newField", updated.Annotations["publicCredentialFields"])
		assert.Equal(t, "secretKey", updated.Annotations["privateCredentialFields"])
		// the schema already matches (including the merged synthetic field), so it must not have been updated
		assert.Len(t, schemaClient.UpdateCalls(), 0)
	})

	t.Run("synthetic fields are excluded from the annotation sync", func(t *testing.T) {
		t.Parallel()

		driverClient := &fakes.NodeDriverInterfaceMock{
			UpdateFunc: func(in1 *v32.NodeDriver) (*v32.NodeDriver, error) {
				return in1, nil
			},
		}
		lifecycle, _ := newTestLifecycle(driverClient, []string{"accessKey"})
		obj := newDriver(map[string]string{
			"publicCredentialFields":  "accessKey",
			"privateCredentialFields": "secretKey",
		})

		result, err := lifecycle.createCredSchema(obj, credFields("accessKey"), []string{"accessKey"}, []string{"secretKey"})

		assert.NoError(t, err)
		assert.Same(t, obj, result)
		// defaultRegion is on the schema spec but must not be written to the annotations
		assert.Len(t, driverClient.UpdateCalls(), 0)
	})

	t.Run("no node driver update when annotations already match as sets", func(t *testing.T) {
		t.Parallel()

		driverClient := &fakes.NodeDriverInterfaceMock{
			UpdateFunc: func(in1 *v32.NodeDriver) (*v32.NodeDriver, error) {
				return in1, nil
			},
		}
		lifecycle, _ := newTestLifecycle(driverClient, []string{"accessKey", "newField"})
		// unsorted annotation value: set-equal to the resolved list
		obj := newDriver(map[string]string{
			"publicCredentialFields":  "newField,accessKey",
			"privateCredentialFields": "secretKey",
		})

		result, err := lifecycle.createCredSchema(obj, credFields("accessKey", "newField"), []string{"accessKey", "newField"}, []string{"secretKey"})

		assert.NoError(t, err)
		assert.Same(t, obj, result)
		assert.Len(t, driverClient.UpdateCalls(), 0)
	})

	t.Run("node driver update failure is propagated", func(t *testing.T) {
		t.Parallel()

		driverClient := &fakes.NodeDriverInterfaceMock{
			UpdateFunc: func(in1 *v32.NodeDriver) (*v32.NodeDriver, error) {
				return nil, fmt.Errorf("update conflict")
			},
		}
		lifecycle, _ := newTestLifecycle(driverClient, []string{"accessKey", "newField"})
		obj := newDriver(map[string]string{
			"publicCredentialFields":  "accessKey",
			"privateCredentialFields": "secretKey",
		})

		result, err := lifecycle.createCredSchema(obj, credFields("accessKey", "newField"), []string{"accessKey", "newField"}, []string{"secretKey"})

		assert.Error(t, err)
		assert.Same(t, obj, result)
	})
}
