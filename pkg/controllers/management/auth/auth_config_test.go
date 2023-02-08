package auth

import (
	"encoding/json"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/rancher/norman/objectclient"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	azuread "github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

func TestCleanupRuns(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		configEnabled      bool
		annotationValue    string
		expectCleanup      bool
		newAnnotationValue string
	}{
		{
			name:               "cleanup runs in disabled unlocked auth config",
			configEnabled:      false,
			annotationValue:    CleanupUnlocked,
			expectCleanup:      true,
			newAnnotationValue: CleanupRancherLocked,
		},
		{
			name:               "no cleanup in disabled auth config without annotation",
			configEnabled:      false,
			annotationValue:    "",
			expectCleanup:      false,
			newAnnotationValue: CleanupRancherLocked,
		},
		{
			name:               "no cleanup in enabled auth config without annotation",
			configEnabled:      true,
			annotationValue:    "",
			expectCleanup:      false,
			newAnnotationValue: CleanupUnlocked,
		},
		{
			name:               "no cleanup in disabled rancher_locked auth config",
			configEnabled:      false,
			annotationValue:    CleanupRancherLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupRancherLocked,
		},
		{
			name:               "no cleanup in disabled user_locked auth config",
			configEnabled:      false,
			annotationValue:    CleanupUserLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUserLocked,
		},
		{
			name:               "no cleanup in enabled unlocked auth config",
			configEnabled:      true,
			annotationValue:    CleanupUnlocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUnlocked,
		},
		{
			name:               "no cleanup in enabled rancher_locked auth config",
			configEnabled:      true,
			annotationValue:    CleanupRancherLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUnlocked,
		},
		{
			name:               "no cleanup in enabled user_locked auth config",
			configEnabled:      true,
			annotationValue:    CleanupUserLocked,
			expectCleanup:      false,
			newAnnotationValue: CleanupUserLocked,
		},
		{
			name:               "no cleanup in disabled auth config with invalid annotation",
			configEnabled:      false,
			annotationValue:    "bad",
			expectCleanup:      false,
			newAnnotationValue: "bad",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        azuread.Name,
					Annotations: map[string]string{CleanupAnnotation: test.annotationValue},
				},
				Enabled: test.configEnabled,
			}

			var service cleanupService
			controller := authConfigController{
				cleanup:                 &service,
				authConfigsUnstructured: newMockAuthConfigClient(config),
			}

			authConfig, err := controller.sync("test", config)
			acObject := authConfig.(*v3.AuthConfig)
			require.NoError(t, err)
			assert.Equal(t, test.newAnnotationValue, acObject.Annotations[CleanupAnnotation])
			assert.Equal(t, test.expectCleanup, service.cleanupCalled)
		})
	}
}

type cleanupService struct {
	cleanupCalled bool
}

func (s *cleanupService) Run(_ *v3.AuthConfig) error {
	s.cleanupCalled = true
	return nil
}

type mockUnstructuredAuthConfig struct {
	config *v3.AuthConfig
}

func (m mockUnstructuredAuthConfig) GetObjectKind() schema.ObjectKind {
	return nil
}

func (m mockUnstructuredAuthConfig) DeepCopyObject() runtime.Object {
	//TODO implement me
	panic("implement me")
}

func (m mockUnstructuredAuthConfig) NewEmptyInstance() runtime.Unstructured {
	//TODO implement me
	panic("implement me")
}

func (m mockUnstructuredAuthConfig) UnstructuredContent() map[string]interface{} {
	var out map[string]any
	b, err := json.Marshal(m.config)
	if err != nil {
		return nil
	}
	err = json.Unmarshal(b, &out)
	if err != nil {
		return nil
	}
	return out
}

func (m mockUnstructuredAuthConfig) SetUnstructuredContent(content map[string]interface{}) {
	b, err := json.Marshal(content)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, m.config)
	if err != nil {
		return
	}
}

func (m mockUnstructuredAuthConfig) IsList() bool {
	//TODO implement me
	panic("implement me")
}

func (m mockUnstructuredAuthConfig) EachListItem(f func(runtime.Object) error) error {
	//TODO implement me
	panic("implement me")
}

type mockAuthConfigClient struct {
	config mockUnstructuredAuthConfig
}

func newMockAuthConfigClient(authConfig *v3.AuthConfig) objectclient.GenericClient {
	return mockAuthConfigClient{config: mockUnstructuredAuthConfig{authConfig}}
}

func (m mockAuthConfigClient) Get(name string, opts metav1.GetOptions) (runtime.Object, error) {
	o := unstructured.Unstructured{}
	o.SetUnstructuredContent(map[string]any{"Object": m.config})
	return &o, nil
}

func (m mockAuthConfigClient) Update(name string, o runtime.Object) (runtime.Object, error) {
	return o, nil
}

func (m mockAuthConfigClient) UnstructuredClient() objectclient.GenericClient {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) GroupVersionKind() schema.GroupVersionKind {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Create(o runtime.Object) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) GetNamespaced(namespace, name string, opts metav1.GetOptions) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) UpdateStatus(name string, o runtime.Object) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) DeleteNamespaced(namespace, name string, opts *metav1.DeleteOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Delete(name string, opts *metav1.DeleteOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) List(opts metav1.ListOptions) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) ListNamespaced(namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) DeleteCollection(deleteOptions *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Patch(name string, o runtime.Object, patchType types.PatchType, data []byte, subresources ...string) (runtime.Object, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) ObjectFactory() objectclient.ObjectFactory {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) ObjectClient() *objectclient.ObjectClient {
	//TODO implement me
	panic("implement me")
}
