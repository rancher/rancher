package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	azuread "github.com/rancher/rancher/pkg/auth/providers/azure/clients"
	controllers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			newAnnotationValue: CleanupRancherLocked,
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

	mockAuthConfig := newMockAuthConfigClient()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var service cleanupService
			controller := authConfigController{
				cleanup:          &service,
				authConfigClient: mockAuthConfig,
			}
			config := &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        azuread.Name,
					Annotations: map[string]string{CleanupAnnotation: test.annotationValue},
				},
				Enabled: test.configEnabled,
			}

			obj, err := controller.sync("test", config)
			require.NoError(t, err)
			assert.Equal(t, test.expectCleanup, service.cleanupCalled)
			assert.Equal(t, test.newAnnotationValue, obj.(*v3.AuthConfig).Annotations[CleanupAnnotation])
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

type mockAuthConfigClient struct {
}

func (m mockAuthConfigClient) Create(_ *v3.AuthConfig) (*v3.AuthConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Update(config *v3.AuthConfig) (*v3.AuthConfig, error) {
	return config, nil
}

func (m mockAuthConfigClient) Delete(_ string, _ *metav1.DeleteOptions) error {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Get(_ string, _ metav1.GetOptions) (*v3.AuthConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) List(_ metav1.ListOptions) (*v3.AuthConfigList, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Watch(_ metav1.ListOptions) (watch.Interface, error) {
	//TODO implement me
	panic("implement me")
}

func (m mockAuthConfigClient) Patch(_ string, _ types.PatchType, _ []byte, _ ...string) (result *v3.AuthConfig, err error) {
	//TODO implement me
	panic("implement me")
}

func newMockAuthConfigClient() controllers.AuthConfigClient {
	return mockAuthConfigClient{}
}
