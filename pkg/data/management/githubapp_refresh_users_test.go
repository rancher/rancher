package management

import (
	"encoding/json"
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestRefreshGitHubAppUsersOnce(t *testing.T) {
	t.Parallel()

	notFound := k8serrors.NewNotFound(schema.GroupResource{Resource: "authconfigs"}, "githubapp")

	tests := []struct {
		name       string
		authConfig *v3.AuthConfig
		getErr     error
		wantPatch  bool
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
		},
		{
			name: "first run",
			authConfig: &v3.AuthConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "githubapp"},
				Enabled:    true,
			},
			wantPatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			mock := fake.NewMockNonNamespacedClientInterface[*v3.AuthConfig, *v3.AuthConfigList](ctrl)
			mock.EXPECT().Get("githubapp", metav1.GetOptions{}).Return(tt.authConfig, tt.getErr)

			if tt.wantPatch {
				mock.EXPECT().Patch("githubapp", types.MergePatchType, gomock.Any()).DoAndReturn(
					func(_ string, _ types.PatchType, data []byte, _ ...string) (*v3.AuthConfig, error) {
						var payload map[string]any
						require.NoError(t, json.Unmarshal(data, &payload))
						metadata, _ := payload["metadata"].(map[string]any)
						annotations, _ := metadata["annotations"].(map[string]any)
						assert.Equal(t, "true", annotations[providerRefreshRequestedAnnotation])
						return tt.authConfig, nil
					},
				)
			}

			RefreshGitHubAppUsersOnce(t.Context(), mock)
		})
	}
}
