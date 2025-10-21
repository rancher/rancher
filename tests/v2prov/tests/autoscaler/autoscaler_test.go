package autoscaler

import (
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func Test_General_RKEMachinePool_Autoscaling_Field_Validation(t *testing.T) {
	client, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	testCases := []struct {
		name          string
		clusterSpec   func() *v1.Cluster
		expectedError string
		shouldSucceed bool
	}{
		{
			name: "Valid - No autoscaling fields",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									EtcdRole:         true,
									WorkerRole:       true,
									ControlPlaneRole: true,
									Quantity:         ptr.To[int32](0),
								},
							},
						},
					},
				}
			},
			shouldSucceed: true,
		},
		{
			name: "Valid - EtcdRole with positive AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
								},
							},
						},
					},
				}
			},
			shouldSucceed: true,
		},
		{
			name: "Valid - ControlPlaneRole with positive AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
								},
							},
						},
					},
				}
			},
			shouldSucceed: true,
		},
		{
			name: "Valid - Both EtcdRole and ControlPlaneRole with positive AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									EtcdRole:           true,
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
								},
							},
						},
					},
				}
			},
			shouldSucceed: true,
		},
		{
			name: "Valid - WorkerRole with zero AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](0),
									AutoscalingMaxSize: ptr.To[int32](3),
								},
							},
						},
					},
				}
			},
			shouldSucceed: true,
		},
		{
			name: "Valid - Valid AutoscalingMinSize and AutoscalingMaxSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
								},
							},
						},
					},
				}
			},
			shouldSucceed: true,
		},
		{
			name: "Valid - AutoscalingMinSize equals AutoscalingMaxSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
								},
							},
						},
					},
				}
			},
			shouldSucceed: true,
		},
		{
			name: "Invalid - EtcdRole with zero AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](0),
									AutoscalingMaxSize: ptr.To[int32](1),
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole or ControlPlaneRole are true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - ControlPlaneRole with zero AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](0),
									AutoscalingMaxSize: ptr.To[int32](1),
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole or ControlPlaneRole are true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Both EtcdRole and ControlPlaneRole with zero AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-etcd-pool",
									EtcdRole:           true,
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](0),
									AutoscalingMaxSize: ptr.To[int32](1),
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole or ControlPlaneRole are true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - AutoscalingMinSize greater than AutoscalingMaxSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](5),
									AutoscalingMaxSize: ptr.To[int32](3),
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be less than or equal to AutoscalingMaxSize when both are non-zero",
			shouldSucceed: false,
		},
		{
			name: "Invalid - EtcdRole with AutoscalingMinSize greater than AutoscalingMaxSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](5),
									AutoscalingMaxSize: ptr.To[int32](3),
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be less than or equal to AutoscalingMaxSize when both are non-zero",
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := cluster.New(client, tc.clusterSpec())
			if err != nil && tc.shouldSucceed {
				t.Fatalf("Expected cluster creation to succeed but got error: %v", err)
			}
			if err == nil && !tc.shouldSucceed {
				t.Fatal("Expected cluster creation to fail but it succeeded")
			}

			if err != nil {
				assert.Contains(t, err.Error(), tc.expectedError,
					"Expected error message not found in actual error")
			}
		})
	}
}
