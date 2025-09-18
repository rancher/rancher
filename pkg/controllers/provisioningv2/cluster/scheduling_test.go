package cluster

import (
	"reflect"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1mocks "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1/mocks"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_updateV1SchedulingCustomization(t *testing.T) {
	tests := []struct {
		name            string
		cluster         v1.Cluster
		expectedErr     bool
		expectChanged   bool
		expectedCluster v1.ClusterSpec
	}{
		{
			name:          "empty cluster",
			cluster:       v1.Cluster{},
			expectChanged: false,
			// expectedCluster: v1.ClusterSpec{},
		},
		{
			name: "no change",
			cluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v1.ClusterSpec{
					FleetAgentDeploymentCustomization: &v1.AgentDeploymentCustomization{
						SchedulingCustomization: &v1.AgentSchedulingCustomization{
							PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{},
							PriorityClass:       &v1.PriorityClassSpec{},
						},
					},
				},
			},
			expectChanged: false,
			// expectedCluster: v1.ClusterSpec{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			clusters := v1mocks.NewMockClusterController(ctrl)
			clusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v1.Cluster) (*v1.Cluster, error) {
				return cluster, nil
			})
			h := handler{
				clusters: clusters,
			}

			outputCluster, err := h.updateV1SchedulingCustomization("", &test.cluster)

			if test.expectedErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !test.expectedErr && err != nil {
				t.Fatalf("did not expect error but got: %v", err)
			}

			if test.expectChanged && reflect.DeepEqual(outputCluster, &test.cluster) {
				t.Fatalf("expected cluster to be changed but it equals the input cluster")
			}
			if !test.expectChanged && !reflect.DeepEqual(outputCluster, &test.cluster) {
				t.Fatalf("expected cluster to be unchanged but it differs from the input cluster")
			}
		})
	}
}
