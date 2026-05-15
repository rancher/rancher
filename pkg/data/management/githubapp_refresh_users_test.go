package management

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRefreshGitHubAppUsersOnce(t *testing.T) {
	t.Parallel()

	notFound := k8serrors.NewNotFound(schema.GroupResource{Resource: "authconfigs"}, "githubapp")

	tests := []struct {
		name            string
		authConfig      *v3.AuthConfig
		getErr          error
		wantUpdateCalls int
		wantAnnotation  bool
	}{
		{
			name:   "provider not configured",
			getErr: notFound,
		},
		{
			name:   "get authconfig error",
			getErr: fmt.Errorf("connection refused"),
		},
		{
			name: "provider not enabled",
			authConfig: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "githubapp"},
				Enabled:    false,
			},
		},
		{
			name: "already ran",
			authConfig: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "githubapp",
					Annotations: map[string]string{providerRefreshRequestedAnnotation: "true"},
				},
				Enabled: true,
			},
			wantAnnotation: true,
		},
		{
			name: "first run",
			authConfig: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "githubapp"},
				Enabled:    true,
			},
			wantUpdateCalls: 1,
			wantAnnotation:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			mock := fake.NewMockNonNamespacedClientInterface[*v3.AuthConfig, *v3.AuthConfigList](ctrl)
			mock.EXPECT().Get("githubapp", metav1.GetOptions{}).Return(tt.authConfig, tt.getErr)

			var updated *v3.AuthConfig
			if tt.wantUpdateCalls > 0 {
				mock.EXPECT().Update(gomock.Any()).DoAndReturn(func(cfg *v3.AuthConfig) (*v3.AuthConfig, error) {
					updated = cfg
					return cfg, nil
				}).Times(tt.wantUpdateCalls)
			}

			RefreshGitHubAppUsersOnce(t.Context(), mock)

			if tt.wantAnnotation {
				source := updated
				if source == nil {
					source = tt.authConfig
				}
				assert.Equal(t, "true", source.Annotations[providerRefreshRequestedAnnotation])
			}
		})
	}
}
