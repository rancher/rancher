package telemetry

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type telInput struct {
	localCluster    *v3.Cluster
	localNodes      []*v3.Node
	managedClusters []*v3.Cluster
	managedNodes    map[ClusterID][]*v3.Node
}

func TestTelemetryManagedClusterCount(t *testing.T) {
	type testcase struct {
		input    telInput
		expected int
	}
	assert := assert.New(t)

	testcases := []testcase{
		{
			input: telInput{
				localCluster:    &v3.Cluster{},
				localNodes:      []*v3.Node{},
				managedClusters: []*v3.Cluster{},
				managedNodes:    map[ClusterID][]*v3.Node{},
			},
			expected: 1,
		},
		{
			input: telInput{
				localCluster: &v3.Cluster{},
				localNodes:   []*v3.Node{},
				managedClusters: []*v3.Cluster{
					{},
				},
				managedNodes: map[ClusterID][]*v3.Node{},
			},
			expected: 2,
		},
		{
			input: telInput{
				localCluster: &v3.Cluster{},
				localNodes:   []*v3.Node{},
				managedClusters: []*v3.Cluster{
					{},
					{},
				},
				managedNodes: map[ClusterID][]*v3.Node{},
			},
			expected: 3,
		},
	}
	for _, tc := range testcases {
		rancherT := newTelemetryImpl(
			"",
			tc.input.localCluster,
			tc.input.localNodes,
			tc.input.managedClusters,
			tc.input.managedNodes,
		)
		assert.Equal(rancherT.ManagedClusterCount(), tc.expected)
	}
}

func TestTelemetryManagedNodes(t *testing.T) {
	type testcase struct {
		input    telInput
		expected int
	}
	assert := assert.New(t)

	testcases := []testcase{
		{
			input: telInput{
				localCluster: &v3.Cluster{},
				localNodes: []*v3.Node{
					{},
				},
				managedClusters: []*v3.Cluster{},
				managedNodes:    map[ClusterID][]*v3.Node{},
			},
			expected: 1,
		},
		{
			input: telInput{
				localCluster: &v3.Cluster{},
				localNodes: []*v3.Node{
					{},
					{},
				},
				managedClusters: []*v3.Cluster{
					{},
				},
				managedNodes: map[ClusterID][]*v3.Node{},
			},
			expected: 2,
		},
	}
	for _, tc := range testcases {
		rancherT := newTelemetryImpl(
			"",
			tc.input.localCluster,
			tc.input.localNodes,
			tc.input.managedClusters,
			tc.input.managedNodes,
		)
		assert.Equal(rancherT.LocalNodeCount(), tc.expected)
	}
}

func TestTelemetryLocalClusterCompute(t *testing.T) {
	type testcase struct {
		input          telInput
		expectedCpu    int
		expectedMem    resource.Quantity
		expectedMemInt int
	}

	assert := assert.New(t)

	testcases := []testcase{
		{
			input: telInput{
				localCluster: &v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "c-wtjw5",
					},
					Status: v3.ClusterStatus{
						Capacity: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1"),
							v1.ResourceMemory: resource.MustParse("8Mi"),
						},
					},
				},
				localNodes:      []*v3.Node{},
				managedClusters: []*v3.Cluster{},
				managedNodes:    map[ClusterID][]*v3.Node{},
			},
			expectedCpu:    1,
			expectedMem:    resource.MustParse("8Mi"),
			expectedMemInt: 8388608,
		},
	}

	for _, tc := range testcases {
		rancherT := newTelemetryImpl(
			"",
			tc.input.localCluster,
			tc.input.localNodes,
			tc.input.managedClusters,
			tc.input.managedNodes,
		)

		clT := rancherT.LocalClusterTelemetry()
		cores, err := clT.CpuCores()
		assert.NoError(err)
		mem, err := clT.MemoryCapacityBytes()
		assert.NoError(err)
		//expectedMem, ok := tc.expectedMem.AsDec().Unscaled()
		//assert.True(ok)
		assert.Equal(tc.expectedCpu, cores, "mismatched cpu cores reported")
		assert.Equal(tc.expectedMemInt, mem, "mismatched memory reported")
	}
}

func TestTelemetryLocalNodeCompute(t *testing.T) {
	type testcase struct {
		input          telInput
		expectedCpu    int
		expectedMem    resource.Quantity
		expectedMemInt int
	}

	assert := assert.New(t)

	testcases := []testcase{
		{
			input: telInput{
				localCluster: &v3.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "c-wtjw5",
					},
					Status: v3.ClusterStatus{
						Capacity: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("1"),
							v1.ResourceMemory: resource.MustParse("8Mi"),
						},
					},
				},
				localNodes: []*v3.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-asfdk7",
							Namespace: "local",
						},
						Status: v3.NodeStatus{
							InternalNodeStatus: v1.NodeStatus{
								Capacity: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("4"),
									v1.ResourceMemory: resource.MustParse("16Gi"),
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "machine-ljsis1",
							Namespace: "local",
						},
						Status: v3.NodeStatus{
							InternalNodeStatus: v1.NodeStatus{
								Capacity: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("22"),
									v1.ResourceMemory: resource.MustParse("7Gi"),
								},
							},
						},
					},
				},
				managedClusters: []*v3.Cluster{},
				managedNodes:    map[ClusterID][]*v3.Node{},
			},
			expectedCpu:    26,
			expectedMem:    resource.MustParse("23Gi"),
			expectedMemInt: 24696061952,
		},
	}

	for _, tc := range testcases {
		rancherT := newTelemetryImpl(
			"",
			tc.input.localCluster,
			tc.input.localNodes,
			tc.input.managedClusters,
			tc.input.managedNodes,
		)
		clT := rancherT.LocalClusterTelemetry()
		totalCores := int(0)
		totalMemory := int(0)

		for _, nodeT := range clT.PerNodeTelemetry() {
			cores, err := nodeT.CpuCores()
			assert.NoError(err)
			memBytes, err := nodeT.MemoryCapacityBytes()
			assert.NoError(err)
			totalCores += cores
			totalMemory += memBytes
		}
		//expectedMem, ok := tc.expectedMem.AsDec().Unscaled()
		//assert.True(ok)
		assert.Equal(tc.expectedCpu, totalCores, "mismatched cpu cores reported")
		assert.Equal(tc.expectedMemInt, totalMemory, "mismatched memory reported")
	}
}

func TestTelemetryClusterCompute(t *testing.T) {
	type testcase struct {
		input          telInput
		expectedCpu    int
		expectedMem    resource.Quantity
		expectedMemInt int
	}
	assert := assert.New(t)

	testcases := []testcase{
		{
			input: telInput{
				localCluster: &v3.Cluster{},
				localNodes: []*v3.Node{
					{},
				},
				managedClusters: []*v3.Cluster{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "c-wtjw5",
						},
						Status: v3.ClusterStatus{
							Capacity: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("1"),
								v1.ResourceMemory: resource.MustParse("8Mi"),
							},
						},
					},
				},
				managedNodes: map[ClusterID][]*v3.Node{},
			},
			expectedCpu:    1,
			expectedMem:    resource.MustParse("8Mi"),
			expectedMemInt: 8388608,
		},
		{
			input: telInput{
				localCluster: &v3.Cluster{},
				localNodes:   []*v3.Node{},
				managedClusters: []*v3.Cluster{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "c-kwerk",
						},
						Status: v3.ClusterStatus{
							Capacity: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("4"),
								v1.ResourceMemory: resource.MustParse("24Gi"),
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "c-pkjsf",
						},
						Status: v3.ClusterStatus{
							Capacity: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("12"),
								v1.ResourceMemory: resource.MustParse("8Gi"),
							},
						},
					},
				},
				managedNodes: map[ClusterID][]*v3.Node{},
			},
			expectedCpu:    16,
			expectedMem:    resource.MustParse("32Gi"),
			expectedMemInt: 34359738368,
		},
	}
	for _, tc := range testcases {
		rancherT := newTelemetryImpl(
			"",
			tc.input.localCluster,
			tc.input.localNodes,
			tc.input.managedClusters,
			tc.input.managedNodes,
		)
		totalCores := int(0)
		totalMem := int(0)
		for _, clT := range rancherT.PerManagedClusterTelemetry() {
			cores, err := clT.CpuCores()
			assert.NoError(err)
			bytes, err := clT.MemoryCapacityBytes()
			assert.NoError(err)
			totalCores += cores
			totalMem += bytes
		}
		// expectedMem, ok := tc.expectedMem.AsDec().Unscaled()
		// assert.True(ok)
		assert.Equal(tc.expectedCpu, totalCores, "mismatched cpu cores reported")
		assert.Equal(tc.expectedMemInt, totalMem, "mismatched memory reported")
	}
}

func TestTelemetryPerNodeCompute(t *testing.T) {
	type testcase struct {
		input          telInput
		expectedCpu    int
		expectedMem    resource.Quantity
		expectedMemInt int
	}
	assert := assert.New(t)

	testcases := []testcase{
		{
			input: telInput{
				localCluster: &v3.Cluster{},
				localNodes:   []*v3.Node{},
				managedClusters: []*v3.Cluster{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "c-kwerk",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "c-pkjsf",
						},
					},
				},
				managedNodes: map[ClusterID][]*v3.Node{
					ClusterID("c-pkjsf"): []*v3.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-klawl",
								Namespace: "c-pkjsf",
							},
							Status: v3.NodeStatus{
								InternalNodeStatus: v1.NodeStatus{
									Capacity: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("2"),
										v1.ResourceMemory: resource.MustParse("9Gi"),
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-pafdkl",
								Namespace: "c-pkjsf",
							},
							Status: v3.NodeStatus{
								InternalNodeStatus: v1.NodeStatus{
									Capacity: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("3"),
										v1.ResourceMemory: resource.MustParse("19Gi"),
									},
								},
							},
						},
					},
					ClusterID("c-kwerk"): []*v3.Node{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-sadfk",
								Namespace: "c-kwerk",
							},
							Status: v3.NodeStatus{
								InternalNodeStatus: v1.NodeStatus{
									Capacity: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("6"),
										v1.ResourceMemory: resource.MustParse("4Gi"),
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-sdfigjn",
								Namespace: "c-kwerk",
							},
							Status: v3.NodeStatus{
								InternalNodeStatus: v1.NodeStatus{
									Capacity: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("25"),
										v1.ResourceMemory: resource.MustParse("9Gi"),
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "machine-vkljasn",
								Namespace: "c-kwerk",
							},
							Status: v3.NodeStatus{
								InternalNodeStatus: v1.NodeStatus{
									Capacity: v1.ResourceList{
										v1.ResourceCPU:    resource.MustParse("99"),
										v1.ResourceMemory: resource.MustParse("128Gi"),
									},
								},
							},
						},
					},
				},
			},
			expectedCpu:    135,
			expectedMem:    resource.MustParse("169Gi"),
			expectedMemInt: 181462368256,
		},
	}
	for _, tc := range testcases {
		rancherT := newTelemetryImpl(
			"",
			tc.input.localCluster,
			tc.input.localNodes,
			tc.input.managedClusters,
			tc.input.managedNodes,
		)
		totalCores := int(0)
		totalMem := int(0)
		for _, clT := range rancherT.PerManagedClusterTelemetry() {
			for _, nodeT := range clT.PerNodeTelemetry() {
				cores, err := nodeT.CpuCores()
				assert.NoError(err)
				memBytes, err := nodeT.MemoryCapacityBytes()
				assert.NoError(err)
				totalCores += cores
				totalMem += memBytes
			}
		}
		// expectedMem, ok := tc.expectedMem.AsDec().Unscaled()
		// assert.True(ok)
		assert.Equal(tc.expectedCpu, totalCores, "mismatched cpu cores reported")
		assert.Equal(tc.expectedMemInt, totalMem, "mismatched memory reported")
	}
}
