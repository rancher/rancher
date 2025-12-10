package cluster

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/pmezard/go-difflib/difflib"
	"go.uber.org/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/features"

	"github.com/rancher/wrangler/v3/pkg/generic/fake"
)

var (
	PreemptLowerPriority = corev1.PreemptLowerPriority
)

func Test_updateV1AgentSchedulingCustomization(t *testing.T) {
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
			name: "has annotation but no customization, expecting removal of annotation and addition of defaults",
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
				},
			},
		},
		{
			name: "has annotation and customization, expecting removal of annotation and no changes to customization",
			cluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v1.ClusterSpec{
					ClusterAgentDeploymentCustomization: &v1.AgentDeploymentCustomization{
						SchedulingCustomization: &v1.AgentSchedulingCustomization{
							PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{
								MaxUnavailable: "50%",
							},
						},
					},
				},
			},
			expectedCluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.ClusterSpec{
					ClusterAgentDeploymentCustomization: &v1.AgentDeploymentCustomization{
						SchedulingCustomization: &v1.AgentSchedulingCustomization{
							PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{
								MaxUnavailable: "50%",
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
			}).AnyTimes()

			h := handler{
				clusters: clusters,
			}

			features.ClusterAgentSchedulingCustomization.Set(true)

			outputCluster, err := h.updateV1AgentSchedulingCustomization(&tt.cluster)

			if tt.expectedErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("did not expect error but got: %v", err)
			}

			if !reflect.DeepEqual(outputCluster, &tt.expectedCluster) {
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
					t.Fatalf("resulting cluster differs from the expected cluster\n%s", text)
				}
			}
		})
	}
}

func Test_updateV1FleetAgentSchedulingCustomization(t *testing.T) {
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
			name: "has annotation but no customization, expecting removal of annotation and addition of defaults",
			cluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						manageFleetSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v1.ClusterSpec{},
			},
			expectedCluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.ClusterSpec{
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
		{
			name: "has annotation and customization, expecting removal of annotation and no changes to customization",
			cluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						manageFleetSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v1.ClusterSpec{
					FleetAgentDeploymentCustomization: &v1.AgentDeploymentCustomization{
						SchedulingCustomization: &v1.AgentSchedulingCustomization{
							PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{
								MaxUnavailable: "50%",
							},
						},
					},
				},
			},
			expectedCluster: v1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.ClusterSpec{
					FleetAgentDeploymentCustomization: &v1.AgentDeploymentCustomization{
						SchedulingCustomization: &v1.AgentSchedulingCustomization{
							PodDisruptionBudget: &v1.PodDisruptionBudgetSpec{
								MaxUnavailable: "50%",
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
			}).AnyTimes()

			h := handler{
				clusters: clusters,
			}

			features.ClusterAgentSchedulingCustomization.Set(true)

			outputCluster, err := h.updateV1FleetAgentSchedulingCustomization(&tt.cluster)

			if tt.expectedErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("did not expect error but got: %v", err)
			}

			if !reflect.DeepEqual(outputCluster, &tt.expectedCluster) {
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
			}
		})
	}
}

func Test_updateV3AgentSchedulingCustomization(t *testing.T) {
	tests := []struct {
		name            string
		cluster         v3.Cluster
		expectedErr     bool
		expectedCluster *v3.Cluster
	}{
		{
			name: "local (legacy) cluster with annotation but no customization, expecting removal of annotation and addition of defaults",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{},
			},
			expectedCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "0",
									MinAvailable:   "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									PreemptionPolicy: &PreemptLowerPriority,
									Value:            1000000000,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "c-cluster (legacy) cluster with annotation but no customization, expecting removal of annotation and addition of defaults",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abcde",
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{},
			},
			expectedCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abcde",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "0",
									MinAvailable:   "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									PreemptionPolicy: &PreemptLowerPriority,
									Value:            1000000000,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "non-legacy cluster, no changes",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{},
			},
			expectedCluster: nil,
		},
		{
			name: "has annotation and customization, expecting removal of annotation and no changes to customization",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "50%",
								},
							},
						},
					},
				},
			},
			expectedCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "50%",
								},
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
			clusters := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](mockCtrl)
			if tt.expectedCluster != nil {
				clusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v3.Cluster) (*v3.Cluster, error) {
					return cluster, nil
				}).AnyTimes()
			}

			h := handler{
				mgmtClusters: clusters,
			}

			features.ClusterAgentSchedulingCustomization.Set(true)

			outputCluster, err := h.updateV3AgentSchedulingCustomization(&tt.cluster)

			if tt.expectedErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("did not expect error but got: %v", err)
			}

			if tt.expectedCluster == nil {
				if outputCluster != nil {
					t.Fatalf("expected nil cluster but got: %v", outputCluster)
				}
				return
			}

			if !reflect.DeepEqual(outputCluster, tt.expectedCluster) {
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
					t.Fatalf("resulting cluster differs from the expected cluster\n%s", text)
				}
			}
		})
	}
}

func Test_updateV3FleetAgentSchedulingCustomization(t *testing.T) {
	tests := []struct {
		name            string
		cluster         v3.Cluster
		expectedErr     bool
		expectedCluster *v3.Cluster
	}{
		{
			name: "local (legacy) cluster with annotation but no customization, expecting removal of annotation and addition of defaults",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
					Annotations: map[string]string{
						manageFleetSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{},
			},
			expectedCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						FleetAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "0",
									MinAvailable:   "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									PreemptionPolicy: &PreemptLowerPriority,
									Value:            999999999,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "c-cluster (legacy) cluster with annotation but no customization, expecting removal of annotation and addition of defaults",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abcde",
					Annotations: map[string]string{
						manageFleetSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{},
			},
			expectedCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "c-abcde",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						FleetAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "0",
									MinAvailable:   "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									PreemptionPolicy: &PreemptLowerPriority,
									Value:            999999999,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "non-legacy cluster, no changes",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-cluster",
					Annotations: map[string]string{
						manageFleetSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{},
			},
			expectedCluster: nil,
		},
		{
			name: "has annotation and customization, expecting removal of annotation and no changes to customization",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
					Annotations: map[string]string{
						manageFleetSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						FleetAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "50%",
								},
							},
						},
					},
				},
			},
			expectedCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						FleetAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "50%",
								},
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
			clusters := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](mockCtrl)
			if tt.expectedCluster != nil {
				clusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v3.Cluster) (*v3.Cluster, error) {
					return cluster, nil
				}).AnyTimes()
			}

			h := handler{
				mgmtClusters: clusters,
			}

			features.ClusterAgentSchedulingCustomization.Set(true)

			outputCluster, err := h.updateV3FleetAgentSchedulingCustomization(&tt.cluster)

			if tt.expectedErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("did not expect error but got: %v", err)
			}

			if tt.expectedCluster == nil {
				if outputCluster != nil {
					t.Fatalf("expected nil cluster but got: %v", outputCluster)
				}
				return
			}

			if !reflect.DeepEqual(outputCluster, tt.expectedCluster) {
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
					t.Fatalf("resulting cluster differs from the expected cluster\n%s", text)
				}
			}
		})
	}
}

func Test_updateV3SchedulingCustomization(t *testing.T) {
	tests := []struct {
		name            string
		cluster         v3.Cluster
		expectedErr     bool
		expectedCluster *v3.Cluster
	}{
		{
			name: "has both annotations and no customizations, expecting both customizations to be added",
			cluster: v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
					Annotations: map[string]string{
						manageSchedulingDefaultsAnn:      "true",
						manageFleetSchedulingDefaultsAnn: "true",
					},
				},
				Spec: v3.ClusterSpec{},
			},
			expectedCluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "local",
				},
				Spec: v3.ClusterSpec{
					ClusterSpecBase: v3.ClusterSpecBase{
						ClusterAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "0",
									MinAvailable:   "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									PreemptionPolicy: &PreemptLowerPriority,
									Value:            1000000000,
								},
							},
						},
						FleetAgentDeploymentCustomization: &v3.AgentDeploymentCustomization{
							SchedulingCustomization: &v3.AgentSchedulingCustomization{
								PodDisruptionBudget: &v3.PodDisruptionBudgetSpec{
									MaxUnavailable: "0",
									MinAvailable:   "1",
								},
								PriorityClass: &v3.PriorityClassSpec{
									PreemptionPolicy: &PreemptLowerPriority,
									Value:            999999999,
								},
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
			clusters := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](mockCtrl)
			if tt.expectedCluster != nil {
				clusters.EXPECT().Update(gomock.Any()).DoAndReturn(func(cluster *v3.Cluster) (*v3.Cluster, error) {
					return cluster, nil
				}).AnyTimes()
			}

			h := handler{
				mgmtClusters: clusters,
			}

			features.ClusterAgentSchedulingCustomization.Set(true)

			outputCluster, err := h.updateV3SchedulingCustomization(tt.name, &tt.cluster)

			if tt.expectedErr && err == nil {
				t.Fatalf("expected error but got none")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("did not expect error but got: %v", err)
			}

			if tt.expectedCluster == nil {
				if outputCluster != nil {
					t.Fatalf("expected nil cluster but got: %v", outputCluster)
				}
				return
			}

			if !reflect.DeepEqual(outputCluster, tt.expectedCluster) {
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
					t.Fatalf("resulting cluster differs from the expected cluster\n%s", text)
				}
			}
		})
	}
}
