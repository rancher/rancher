package telemetry

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSccPayload(t *testing.T) {
	type testcase struct {
		input            telInput
		expectedSystems  []SccSystem
		expectedClusters []SccCluster
		err              error
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
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "c-kwpow",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "c-weoriyu",
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
										v1.ResourceCPU:    resource.MustParse("6"),
										v1.ResourceMemory: resource.MustParse("4Gi"),
									},
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
									},
								},
							},
						},
					},
					ClusterID("c-kwpow"): []*v3.Node{
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "amd64",
									},
								},
							},
						},
					},
					ClusterID("c-weoriyu"): []*v3.Node{
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "arm64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "arm64",
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
									NodeInfo: v1.NodeSystemInfo{
										Architecture: "arm64",
									},
								},
							},
						},
					},
				},
			},
			expectedSystems: []SccSystem{
				{
					Arch:   "amd64",
					Cpu:    6,
					Memory: 4096,
					Count:  3,
				},
				{
					Arch:   "amd64",
					Cpu:    25,
					Memory: 9216,
					Count:  2,
				},
				{
					Arch:   "amd64",
					Cpu:    99,
					Memory: 131072,
					Count:  2,
				},
				{
					Arch:   "amd64",
					Cpu:    3,
					Memory: 19456,
					Count:  1,
				},
				{
					Arch:   "arm64",
					Cpu:    6,
					Memory: 4096,
					Count:  1,
				},
				{
					Arch:   "arm64",
					Cpu:    99,
					Memory: 131072,
					Count:  1,
				},
				{
					Arch:   "arm64",
					Cpu:    25,
					Memory: 9216,
					Count:  1,
				},
			},
			expectedClusters: []SccCluster{
				{
					Count:    1,
					Upstream: true,
					Nodes:    0,
				},
				{
					Count:    1,
					Upstream: false,
					Nodes:    2,
				},
				{
					Count:    3,
					Upstream: false,
					Nodes:    3,
				},
			},
			err: nil,
		},
	}

	for _, tc := range testcases {
		rancherT := newTelemetryImpl(
			"",
			"",
			"",
			"",
			tc.input.localCluster,
			tc.input.localNodes,
			tc.input.managedClusters,
			tc.input.managedNodes,
		)
		payload, err := GenerateSCCPayload(rancherT)
		assert.Equal(err, tc.err)
		assert.ElementsMatch(tc.expectedClusters, payload.ManagedClusters)
		assert.ElementsMatch(tc.expectedSystems, payload.ManagedSystems)
	}
}
