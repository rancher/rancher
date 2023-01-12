package clusterprovisioner

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/version"
)

func addClusterCondition(cluster *v3.Cluster) *v3.Cluster {
	v3.ClusterConditionHelmReleasesMigrated.True(cluster)
	return cluster
}

func TestShouldCleanHelmReleases(t *testing.T) {
	tests := []struct {
		name           string
		featureEnabled bool
		cluster        *v3.Cluster
		expectedResult bool
	}{
		{
			name:           "Should not clean up when cluster is not 1.25",
			featureEnabled: true,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{Version: &version.Info{GitVersion: "v1.24.8"}},
			},
			expectedResult: false,
		},
		{
			name:           "Should not clean up when feature is disabled",
			featureEnabled: false,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{Version: &version.Info{GitVersion: "v1.25.2"}},
			},
			expectedResult: false,
		},
		{
			name:           "Should not clean up when version is unavailable",
			featureEnabled: true,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{Version: nil},
			},
			expectedResult: false,
		},
		{
			name:           "Should not clean up when the cleaned up condition is set",
			featureEnabled: true,
			cluster: addClusterCondition(&v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{GitVersion: "v1.25.2"},
				},
			}),
			expectedResult: false,
		},
		{
			name:           "Should clean up when the feature is enabled, cluster condition is unset, and version is 1.25",
			featureEnabled: true,
			cluster: &v3.Cluster{
				Status: v3.ClusterStatus{
					Version: &version.Info{GitVersion: "v1.25.2"},
				},
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualResult := shouldCleanHelmReleases(tt.featureEnabled, tt.cluster)

			assert.Equal(t, tt.expectedResult, actualResult)
		})
	}
}
