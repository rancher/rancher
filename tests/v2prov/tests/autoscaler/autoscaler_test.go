package autoscaler

import (
	"testing"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-no-autoscaling",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									EtcdRole:         true,
									WorkerRole:       true,
									ControlPlaneRole: true,
									Quantity:         ptr.To[int32](0),
									NodeConfig:       &corev1.ObjectReference{},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-etcd-autoscaling-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
									NodeConfig:         &corev1.ObjectReference{},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-controlplane-autoscaling-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
									NodeConfig:         &corev1.ObjectReference{},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-etcd-controlplane-autoscaling-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									EtcdRole:           true,
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
									NodeConfig:         &corev1.ObjectReference{},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-worker-zero-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](0),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-worker-autoscaling-range",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-worker-equal-min-max-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](1),
									NodeConfig:         &corev1.ObjectReference{},
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
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-etcd-zero-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](0),
									AutoscalingMaxSize: ptr.To[int32](1),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole is true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - ControlPlaneRole with zero AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-controlplane-zero-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](0),
									AutoscalingMaxSize: ptr.To[int32](1),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when ControlPlaneRole is true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Both EtcdRole and ControlPlaneRole with zero AutoscalingMinSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-etcd-controlplane-zero-min-size",
					},
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
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole is true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - AutoscalingMinSize greater than AutoscalingMaxSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-worker-min-max-size-reversed",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](5),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be less than or equal to AutoscalingMaxSize when both are non-nil",
			shouldSucceed: false,
		},
		{
			name: "Invalid - EtcdRole with AutoscalingMinSize greater than AutoscalingMaxSize",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-etcd-min-max-size-reversed",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](5),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be less than or equal to AutoscalingMaxSize when both are non-nil",
			shouldSucceed: false,
		},
		{
			name: "Invalid - EtcdRole with AutoscalingMinSize greater than AutoscalingMaxSize and Max set to zero",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-etcd-min-max-size-reversed-max-at-zero",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](5),
									AutoscalingMaxSize: ptr.To[int32](0),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize must be less than or equal to AutoscalingMaxSize when both are non-nil",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Create WorkerRole with AutoscalingMinSize present but AutoscalingMaxSize nil",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-create-worker-min-size-max-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: nil,
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Create WorkerRole with AutoscalingMaxSize present but AutoscalingMinSize nil",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-create-worker-max-size-min-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: nil,
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Create EtcdRole with AutoscalingMinSize present but AutoscalingMaxSize nil",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-create-etcd-min-size-max-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: nil,
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Create EtcdRole with AutoscalingMaxSize present but AutoscalingMinSize nil",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-create-etcd-max-size-min-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: nil,
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Create ControlPlaneRole with AutoscalingMinSize present but AutoscalingMaxSize nil",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-create-controlplane-min-size-max-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: nil,
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Create ControlPlaneRole with AutoscalingMaxSize present but AutoscalingMinSize nil",
			clusterSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-create-controlplane-max-size-min-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: nil,
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
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

func Test_General_RKEMachinePool_Autoscaling_Update_Field_Validation(t *testing.T) {
	client, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	testCases := []struct {
		name          string
		initialSpec   func() *v1.Cluster
		updateSpec    func(*v1.Cluster) *v1.Cluster
		expectedError string
		shouldSucceed bool
	}{
		{
			name: "Valid - Update from no autoscaling to valid autoscaling",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-update-no-to-autoscaling",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									EtcdRole:         true,
									WorkerRole:       true,
									ControlPlaneRole: true,
									Quantity:         ptr.To[int32](0),
									NodeConfig:       &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](1)
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMaxSize = ptr.To[int32](3)
				return c
			},
			shouldSucceed: true,
		},
		{
			name: "Invalid - Update from valid to invalid ControlPlaneRole with zero AutoscalingMinSize",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-controlplane-zero-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](0)
				return c
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when ControlPlaneRole is true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid EtcdRole with zero AutoscalingMinSize",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-etcd-zero-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](0)
				return c
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole is true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid AutoscalingMinSize > AutoscalingMaxSize",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-min-max-size-reversed",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](5)
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMaxSize = ptr.To[int32](3)
				return c
			},
			expectedError: "AutoscalingMinSize must be less than or equal to AutoscalingMaxSize when both are non-nil",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid AutoscalingMinSize > AutoscalingMaxSize and max = 0",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-min-max-size-reversed",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](5)
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMaxSize = ptr.To[int32](0)
				return c
			},
			expectedError: "AutoscalingMinSize must be less than or equal to AutoscalingMaxSize when both are non-nil",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid ControlPlaneRole with negative AutoscalingMinSize",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-controlplane-negative-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](-1)
				return c
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when ControlPlaneRole is true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid EtcdRole with negative AutoscalingMinSize",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-etcd-negative-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](-1)
				return c
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole is true",
			shouldSucceed: false,
		},
		{
			name: "Valid - Update from valid to valid with only AutoscalingMaxSize changed",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-update-max-size-only",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMaxSize = ptr.To[int32](5)
				return c
			},
			shouldSucceed: true,
		},
		{
			name: "Valid - Update from valid to valid with only AutoscalingMinSize changed",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "valid-update-min-size-only",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](2)
				return c
			},
			shouldSucceed: true,
		},
		{
			name: "Invalid - Update from valid to invalid with both ControlPlaneRole and EtcdRole with zero AutoscalingMinSize",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-both-roles-zero-min-size",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-etcd-pool",
									EtcdRole:           true,
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = ptr.To[int32](0)
				return c
			},
			expectedError: "AutoscalingMinSize must be greater than 0 when EtcdRole is true",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid WorkerRole with AutoscalingMinSize present but AutoscalingMaxSize nil",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-worker-min-size-max-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMaxSize = nil
				return c
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid WorkerRole with AutoscalingMaxSize present but AutoscalingMinSize nil",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-worker-max-size-min-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "worker-pool",
									WorkerRole:         true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = nil
				return c
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid EtcdRole with AutoscalingMinSize present but AutoscalingMaxSize nil",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-etcd-min-size-max-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMaxSize = nil
				return c
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid EtcdRole with AutoscalingMaxSize present but AutoscalingMinSize nil",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-etcd-max-size-min-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "etcd-pool",
									EtcdRole:           true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = nil
				return c
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid ControlPlaneRole with AutoscalingMinSize present but AutoscalingMaxSize nil",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-controlplane-min-size-max-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMaxSize = nil
				return c
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
		{
			name: "Invalid - Update from valid to invalid ControlPlaneRole with AutoscalingMaxSize present but AutoscalingMinSize nil",
			initialSpec: func() *v1.Cluster {
				return &v1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "invalid-update-controlplane-max-size-min-nil",
					},
					Spec: v1.ClusterSpec{
						RKEConfig: &v1.RKEConfig{
							MachinePools: []v1.RKEMachinePool{
								{
									Name:               "control-plane-pool",
									ControlPlaneRole:   true,
									Quantity:           ptr.To[int32](0),
									AutoscalingMinSize: ptr.To[int32](1),
									AutoscalingMaxSize: ptr.To[int32](3),
									NodeConfig:         &corev1.ObjectReference{},
								},
							},
						},
					},
				}
			},
			updateSpec: func(c *v1.Cluster) *v1.Cluster {
				c.Spec.RKEConfig.MachinePools[0].AutoscalingMinSize = nil
				return c
			},
			expectedError: "AutoscalingMinSize and AutoscalingMaxSize must both be set if enabling cluster-autoscaling",
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create initial cluster
			initialCluster := tc.initialSpec()
			createdCluster, err := cluster.New(client, initialCluster)
			if err != nil {
				t.Fatalf("Failed to create initial cluster: %v", err)
			}

			// using a custom backoff - retrying with retry.DefaultBackoff led to some flakiness.
			err = retry.RetryOnConflict(wait.Backoff{
				Duration: 100 * time.Millisecond,
				Jitter:   0.1,
				Steps:    5,
				Cap:      10 * time.Second,
			},
				func() error {
					clusterFromAPIServer, err := client.Provisioning.Cluster().Get(createdCluster.Namespace, createdCluster.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					// Update the cluster with the new spec
					updatedCluster := tc.updateSpec(clusterFromAPIServer.DeepCopy())
					// Update the cluster in Kubernetes, retrying on conflict
					_, err = client.Provisioning.Cluster().Update(updatedCluster)
					return err
				})
			if err != nil && tc.shouldSucceed {
				t.Fatalf("Expected cluster update to succeed but got error: %v", err)
			}
			if err == nil && !tc.shouldSucceed {
				t.Fatal("Expected cluster update to fail but it succeeded")
			}

			if err != nil {
				assert.Contains(t, err.Error(), tc.expectedError,
					"Expected error message not found in actual error")
			}
		})
	}
}
