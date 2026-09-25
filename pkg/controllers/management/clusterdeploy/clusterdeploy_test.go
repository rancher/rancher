package clusterdeploy

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/imported"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_clusterAgentCRTName(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *apimgmtv3.Cluster
		expected string
	}{
		{
			// v2prov: the CAPR planner also renders this manifest and picks default-token,
			// so this controller has to agree or its apply rolls the agent.
			name: "v2prov cluster uses the default token",
			cluster: &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{
				Name:        "c-m-abc12",
				Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
			}},
			expected: capr.DefaultClusterRegistrationTokenName,
		},
		{
			// The choice must not depend on Status.Driver, which flips to "imported" when
			// the cluster agent's tunnel is authorized.
			name: "v2prov cluster before the agent connects",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "c-m-abc12",
					Annotations: map[string]string{imported.AdministratedAnnotation: "true"},
				},
				Status: apimgmtv3.ClusterStatus{Driver: ""},
			},
			expected: capr.DefaultClusterRegistrationTokenName,
		},
		{
			name: "imported cluster keeps the system token",
			cluster: &apimgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-abc12"},
				Status:     apimgmtv3.ClusterStatus{Driver: "rke2"},
			},
			expected: systemCRTName,
		},
		{
			name: "CAPI/turtles cluster keeps the system token",
			cluster: &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{
				Name:        "c-m-abc12",
				Annotations: map[string]string{imported.AdministratedAnnotation: "false"},
			}},
			expected: systemCRTName,
		},
		{
			name:     "hosted cluster keeps the system token",
			cluster:  &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abc12"}},
			expected: systemCRTName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, clusterAgentCRTName(tt.cluster))
		})
	}
}
