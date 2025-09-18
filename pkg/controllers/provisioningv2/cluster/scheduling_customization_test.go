package cluster

import (
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/pmezard/go-difflib/difflib"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
)

var (
	PreemptLowerPriority = corev1.PreemptLowerPriority
)

func Test_updateV1SchedulingCustomization(t *testing.T) {
	tests := []struct {
		name            string
		cluster         v1.Cluster
		expectedErr     bool
		expectedCluster v1.Cluster
	}{
		{
			name:            "empty cluster missed annotation, no change expected",
			cluster:         v1.Cluster{},
			expectedCluster: v1.Cluster{},
		},
		{
			name: "has annotation but no customization, annotation removed",
			cluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v1.ClusterSpec{},
			},
			expectedCluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.ClusterSpec{
					ClusterAgentDeploymentCustomization: &v1.AgentDeploymentCustomization{
						SchedulingCustomization: &v1.AgentSchedulingCustomization{
							PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{
								MaxUnavailable: "0",
								MinAvailable:   "1",
							},
							PriorityClass: &v1.PriorityClassSpec{
								PreemptionPolicy: &PreemptLowerPriority,
								Value:            1000000000,
							},
						},
					},
					FleetAgentDeploymentCustomization: &v1.AgentDeploymentCustomization{
						SchedulingCustomization: &v1.AgentSchedulingCustomization{
							PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{
								MaxUnavailable: "0",
								MinAvailable:   "1",
							},
							PriorityClass: &v1.PriorityClassSpec{
								PreemptionPolicy: &PreemptLowerPriority,
								Value:            999999999,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			clusters := fake.NewMockControllerInterface[*v1.Cluster, *v1.ClusterList](mockCtrl)
			clusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v1.Cluster) (*v1.Cluster, error) {
				return cluster, nil
			}).Times(0).MaxTimes(1)

			h := handler{
				clusters: clusters,
			}

			features.ClusterAgentSchedulingCustomization.Set(true)

			outputCluster, err := h.updateV1SchedulingCustomization("", &tt.cluster)

			if tt.expectedErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("did not expect error but got: %v", err)
			}

			out, err := yaml.Marshal(outputCluster)
			if err != nil {
				t.Fatalf("failed to marshal input cluster: %v", err)
			}
			expected, err := yaml.Marshal(tt.expectedCluster)
			if err != nil {
				t.Fatalf("failed to marshal output cluster: %v", err)
			}
			diff := difflib.UnifiedDiff{
				A:        difflib.SplitLines(string(out)),
				B:        difflib.SplitLines(string(expected)),
				FromFile: "actual",
				ToFile:   "expected",
				Context:  3,
			}
			text, err := difflib.GetUnifiedDiffString(diff)
			if err != nil {
				t.Fatalf("failed to get diff string: %v", err)
			}
			if text != "" {
				fmt.Printf("Diff:\n%s\n", text)
				t.Fatalf("resulting cluster differs from the expected cluster\n%s", text)
			}
		})
	}
}
