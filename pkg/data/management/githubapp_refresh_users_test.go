package management

import (
	"encoding/json"
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	clientv3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestRefreshGitHubAppUsersOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		listItems      []v3.AuthConfig
		listErr        error
		wantPatchNames []string
	}{
		{
			name:    "list error",
			listErr: fmt.Errorf("connection refused"),
		},
		{
			name: "no githubapp configs",
			listItems: []v3.AuthConfig{
				{ObjectMeta: metav1.ObjectMeta{Name: "github"}, Type: "githubConfig", Enabled: true},
			},
		},
		{
			name: "provider not enabled",
			listItems: []v3.AuthConfig{
				{ObjectMeta: metav1.ObjectMeta{Name: "githubapp"}, Type: clientv3.GithubAppConfigType, Enabled: false},
			},
		},
		{
			name: "already ran",
			listItems: []v3.AuthConfig{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "githubapp",
						Annotations: map[string]string{providerRefreshRequestedAnnotation: "true"},
					},
					Type:    clientv3.GithubAppConfigType,
					Enabled: true,
				},
			},
		},
		{
			name: "first run",
			listItems: []v3.AuthConfig{
				{ObjectMeta: metav1.ObjectMeta{Name: "githubapp"}, Type: clientv3.GithubAppConfigType, Enabled: true},
			},
			wantPatchNames: []string{"githubapp"},
		},
		{
			name: "multiple configs, first run",
			listItems: []v3.AuthConfig{
				{ObjectMeta: metav1.ObjectMeta{Name: "githubapp"}, Type: clientv3.GithubAppConfigType, Enabled: true},
				{ObjectMeta: metav1.ObjectMeta{Name: "githubapp2"}, Type: clientv3.GithubAppConfigType, Enabled: true},
			},
			wantPatchNames: []string{"githubapp", "githubapp2"},
		},
		{
			name: "multiple configs, partial refresh",
			listItems: []v3.AuthConfig{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "githubapp",
						Annotations: map[string]string{providerRefreshRequestedAnnotation: "true"},
					},
					Type:    clientv3.GithubAppConfigType,
					Enabled: true,
				},
				{ObjectMeta: metav1.ObjectMeta{Name: "githubapp2"}, Type: clientv3.GithubAppConfigType, Enabled: true},
			},
			wantPatchNames: []string{"githubapp2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			mock := fake.NewMockNonNamespacedClientInterface[*v3.AuthConfig, *v3.AuthConfigList](ctrl)
			mock.EXPECT().List(metav1.ListOptions{}).Return(&v3.AuthConfigList{Items: tt.listItems}, tt.listErr)

			for _, name := range tt.wantPatchNames {
				mock.EXPECT().Patch(name, types.MergePatchType, gomock.Any()).DoAndReturn(
					func(_ string, _ types.PatchType, data []byte, _ ...string) (*v3.AuthConfig, error) {
						var payload map[string]any
						require.NoError(t, json.Unmarshal(data, &payload))
						metadata, _ := payload["metadata"].(map[string]any)
						annotations, _ := metadata["annotations"].(map[string]any)
						assert.Equal(t, "true", annotations[providerRefreshRequestedAnnotation])
						return &v3.AuthConfig{}, nil
					},
				)
			}

			RefreshGitHubAppUsersOnce(t.Context(), mock)
		})
	}
}
