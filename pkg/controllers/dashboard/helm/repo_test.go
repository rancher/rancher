package helm

import (
	"slices"
	"testing"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterRepoOnChangeDownloadCondition(t *testing.T) {
	tests := []struct {
		name                     string
		hasDownloadCondition     bool
		expectedStatusUpdates    int
		expectedConditionMessage string
	}{
		{
			name:                     "sets downloading condition when absent",
			expectedStatusUpdates:    1,
			expectedConditionMessage: "downloading",
		},
		{
			name:                     "does not reset existing download condition",
			hasDownloadCondition:     true,
			expectedStatusUpdates:    1,
			expectedConditionMessage: "previous download",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			clusterRepos := fake.NewMockNonNamespacedControllerInterface[*catalog.ClusterRepo, *catalog.ClusterRepoList](ctrl)
			clusterRepo := &catalog.ClusterRepo{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-repo",
					ResourceVersion: "1",
					Generation:      1,
				},
				Status: catalog.RepoStatus{
					ObservedGeneration: 1,
				},
			}
			if tt.hasDownloadCondition {
				clusterRepo.Status.Conditions = []genericcondition.GenericCondition{
					{
						Type:    string(repoCondition),
						Status:  corev1.ConditionFalse,
						Reason:  "previous",
						Message: "previous download",
					},
				}
			}

			var updatedStatuses []catalog.RepoStatus

			clusterRepos.EXPECT().
				Get(clusterRepo.Name, metav1.GetOptions{}).
				Return(clusterRepo, nil)

			clusterRepos.EXPECT().
				UpdateStatus(gomock.Any()).
				DoAndReturn(func(updated *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
					updatedStatuses = append(updatedStatuses, *updated.Status.DeepCopy())
					return updated, nil
				}).
				Times(tt.expectedStatusUpdates)

			clusterRepos.EXPECT().EnqueueAfter(clusterRepo.Name, gomock.Any()).Times(1)

			handler := &repoHandler{clusterRepos: clusterRepos}
			_, err := handler.ClusterRepoOnChange(clusterRepo.Name, clusterRepo)

			assert.NoError(t, err)

			foundDownloadingCondition := slices.ContainsFunc(updatedStatuses, func(s catalog.RepoStatus) bool {
				return slices.ContainsFunc(s.Conditions, func(c genericcondition.GenericCondition) bool {
					return c.Message == "downloading"
				})
			})

			if tt.hasDownloadCondition {
				assert.False(t, foundDownloadingCondition)
			} else {
				assert.True(t, foundDownloadingCondition)
			}
		})
	}
}
