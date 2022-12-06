package projectresources

import (
	"testing"

	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

func Test_formatter(t *testing.T) {
	testSchemas := types.EmptyAPISchemas()
	testSchemas.MustAddSchema(types.APISchema{Schema: &schemas.Schema{ID: "namespace"}})
	testSchemas.MustAddSchema(types.APISchema{Schema: &schemas.Schema{ID: "pod"}})
	testSchemas.MustAddSchema(types.APISchema{Schema: &schemas.Schema{ID: "rbac.authorization.io.clusterrole"}})
	testSchemas.MustAddSchema(types.APISchema{Schema: &schemas.Schema{ID: "apps.deployment"}})
	apiOp := &types.APIRequest{
		Schemas:       testSchemas,
		AccessControl: mockAccessControl{},
	}
	tests := []struct {
		name     string
		resource *types.RawResource
		want     map[string]string
	}{
		{
			name: "steve url for core global resource", // there shouldn't ever be schemas for non-namespaced resources but just for completeness
			resource: &types.RawResource{
				Links: map[string]string{
					"self": "https://example/v1/resources.project.cattle.io.namespaces/testns1",
				},
				Type: "resources.project.cattle.io.namespaces",
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
						},
					},
				},
			},
			want: map[string]string{
				"self":   "https://example/v1/namespaces/testns1",
				"update": "https://example/v1/namespaces/testns1",
				"delete": "https://example/v1/namespaces/testns1",
			},
		},
		{
			name: "steve url for core namespaced resource",
			resource: &types.RawResource{
				Links: map[string]string{
					"self": "https://example/v1/resources.project.cattle.io.pods/testns1/testpod1",
				},
				Type: "resources.project.cattle.io.pods",
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
						},
					},
				},
			},
			want: map[string]string{
				"self":   "https://example/v1/pods/testns1/testpod1",
				"update": "https://example/v1/pods/testns1/testpod1",
				"delete": "https://example/v1/pods/testns1/testpod1",
			},
		},
		{
			name: "steve url for non-core global resource",
			resource: &types.RawResource{
				Links: map[string]string{
					"self": "https://example/v1/resources.project.cattle.io.rbac.authorization.io.clusterroles/testcr1",
				},
				Type: "resources.project.cattle.io.rbac.authorization.io.clusterroles",
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "rbac.authorization.io/v1",
						},
					},
				},
			},
			want: map[string]string{
				"self":   "https://example/v1/rbac.authorization.io.clusterroles/testcr1",
				"update": "https://example/v1/rbac.authorization.io.clusterroles/testcr1",
				"delete": "https://example/v1/rbac.authorization.io.clusterroles/testcr1",
			},
		},
		{
			name: "steve url for non-core namespaced resource",
			resource: &types.RawResource{
				Links: map[string]string{
					"self": "https://example/v1/resources.project.cattle.io.apps.deployments/testns1/testdeploy1",
				},
				Type: "resources.project.cattle.io.apps.deployments",
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
						},
					},
				},
			},
			want: map[string]string{
				"self":   "https://example/v1/apps.deployments/testns1/testdeploy1",
				"update": "https://example/v1/apps.deployments/testns1/testdeploy1",
				"delete": "https://example/v1/apps.deployments/testns1/testdeploy1",
			},
		},
		{
			name: "k8s url for core global resource",
			resource: &types.RawResource{
				Links: map[string]string{
					"view": "https://example/apis/resources.project.cattle.io/v1alpha1/namespaces/testns1",
				},
				Type: "resources.project.cattle.io.namespaces",
				Schema: &types.APISchema{
					Schema: &schemas.Schema{
						Attributes: map[string]interface{}{
							"resource": "namespaces",
						},
					},
				},
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
						},
					},
				},
			},
			want: map[string]string{
				"view": "https://example/api/v1/namespaces/testns1",
			},
		},
		{
			name: "k8s url for core namespaced resource",
			resource: &types.RawResource{
				Links: map[string]string{
					"view": "https://example/apis/resources.project.cattle.io/v1alpha1/namespaces/testns1/pods/testpod1",
				},
				Type: "resources.project.cattle.io.pods",
				Schema: &types.APISchema{
					Schema: &schemas.Schema{
						Attributes: map[string]interface{}{
							"resource": "pods",
						},
					},
				},
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
						},
					},
				},
			},
			want: map[string]string{
				"view": "https://example/api/v1/namespaces/testns1/pods/testpod1",
			},
		},
		{
			name: "k8s url for non-core global resource",
			resource: &types.RawResource{
				Links: map[string]string{
					"view": "https://example/apis/resources.project.cattle.io/v1alpha1/rbac.authorization.io.clusterroles/testcr1",
				},
				Type: "resources.project.cattle.io.rbac.authorization.io.clusterroles",
				Schema: &types.APISchema{
					Schema: &schemas.Schema{
						Attributes: map[string]interface{}{
							"resource": "rbac.authorization.io.clusterroles",
						},
					},
				},
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "rbac.authorization.io/v1",
						},
					},
				},
			},
			want: map[string]string{
				"view": "https://example/apis/rbac.authorization.io/v1/clusterroles/testcr1",
			},
		},
		{
			name: "k8s url for non-core namespaced resource",
			resource: &types.RawResource{
				Links: map[string]string{
					"view": "https://example/apis/resources.project.cattle.io/v1alpha1/namespaces/testns1/apps.deployments/testdeploy1",
				},
				Type: "resources.project.cattle.io.apps.deployments",
				Schema: &types.APISchema{
					Schema: &schemas.Schema{
						Attributes: map[string]interface{}{
							"resource": "apps.deployments",
						},
					},
				},
				APIObject: types.APIObject{
					Object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
						},
					},
				},
			},
			want: map[string]string{
				"view": "https://example/apis/apps/v1/namespaces/testns1/deployments/testdeploy1",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatter(apiOp, test.resource)
			assert.Equal(t, test.want, test.resource.Links)
		})
	}
}

func Test_checkServerConfig(t *testing.T) {
	tests := []struct {
		name string
		data *corev1.ConfigMap
		want bool
	}{
		{
			name: "no configmap",
			data: nil,
			want: false,
		},
		{
			name: "unconfigured configmap",
			data: &corev1.ConfigMap{
				Data: map[string]string{},
			},
			want: false,
		},
		{
			name: "configured configmap",
			data: &corev1.ConfigMap{
				Data: map[string]string{
					"requestheader-client-ca-file": "ca bytes",
				},
			},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mock := &mockConfigMapClient{
				configMap: test.data,
			}
			got := checkServerConfig(mock)
			assert.Equal(t, test.want, got)
		})
	}
}

type mockAccessControl struct{}

func (a mockAccessControl) CanAction(apiOp *types.APIRequest, schema *types.APISchema, name string) error {
	return nil
}

func (a mockAccessControl) CanCreate(apiOp *types.APIRequest, schema *types.APISchema) error {
	return nil
}

func (a mockAccessControl) CanList(apiOp *types.APIRequest, schema *types.APISchema) error {
	return nil
}

func (a mockAccessControl) CanGet(apiOp *types.APIRequest, schema *types.APISchema) error {
	return nil
}

func (a mockAccessControl) CanUpdate(apiOp *types.APIRequest, obj types.APIObject, schema *types.APISchema) error {
	return nil
}

func (a mockAccessControl) CanDelete(apiOp *types.APIRequest, obj types.APIObject, schema *types.APISchema) error {
	return nil
}

func (a mockAccessControl) CanWatch(apiOp *types.APIRequest, schema *types.APISchema) error {
	return nil
}

func (a mockAccessControl) CanDo(apiOp *types.APIRequest, resource, verb, namespace, name string) error {
	return nil
}

type mockConfigMapClient struct {
	configMap *corev1.ConfigMap
}

func (c *mockConfigMapClient) Get(_, _ string, _ metav1.GetOptions) (*corev1.ConfigMap, error) {
	if c.configMap != nil {
		return c.configMap, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (c *mockConfigMapClient) Create(*corev1.ConfigMap) (*corev1.ConfigMap, error) {
	panic("not implemented")
}
func (c *mockConfigMapClient) Update(*corev1.ConfigMap) (*corev1.ConfigMap, error) {
	panic("not implemented")
}
func (c *mockConfigMapClient) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	panic("not implemented")
}
func (c *mockConfigMapClient) List(_ string, _ metav1.ListOptions) (*corev1.ConfigMapList, error) {
	panic("not implemented")
}
func (c *mockConfigMapClient) Watch(_ string, _ metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}
func (c *mockConfigMapClient) Patch(_, _ string, _ k8stypes.PatchType, _ []byte, _ ...string) (*corev1.ConfigMap, error) {
	panic("not implemented")
}
