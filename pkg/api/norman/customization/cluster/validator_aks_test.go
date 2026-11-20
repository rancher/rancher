package cluster

import (
	"testing"

	armcontainerservice "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v9"
	aksv1 "github.com/rancher/aks-operator/pkg/apis/aks.cattle.io/v1"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestValidateAKSNetworkPolicy(t *testing.T) {
	validator := &Validator{}

	tests := []struct {
		name          string
		networkPolicy string   // set on clusterSpec.AKSConfig
		pniEnabled    bool     // EnableNetworkPolicy
		prevPolicy    string   // set on prevCluster upstream spec (only used when networkPolicy is "")
		wantErr       bool
	}{
		{
			name:          "azure policy with PNI enabled",
			networkPolicy: string(armcontainerservice.NetworkPolicyAzure),
			pniEnabled:    true,
			wantErr:       false,
		},
		{
			name:          "calico policy with PNI enabled",
			networkPolicy: string(armcontainerservice.NetworkPolicyCalico),
			pniEnabled:    true,
			wantErr:       false,
		},
		{
			name:          "cilium policy with PNI enabled",
			networkPolicy: string(armcontainerservice.NetworkPolicyCilium),
			pniEnabled:    true,
			wantErr:       false,
		},
		{
			name:          "none policy with PNI enabled",
			networkPolicy: string(armcontainerservice.NetworkPolicyNone),
			pniEnabled:    true,
			wantErr:       true,
		},
		{
			name:          "cilium policy with PNI disabled",
			networkPolicy: string(armcontainerservice.NetworkPolicyCilium),
			pniEnabled:    false,
			wantErr:       false,
		},
		{
			name:          "no policy set skips validation",
			networkPolicy: "",
			pniEnabled:    true,
			wantErr:       false,
		},
		{
			name:       "cilium from upstream spec with PNI enabled",
			prevPolicy: string(armcontainerservice.NetworkPolicyCilium),
			pniEnabled: true,
			wantErr:    false,
		},
		{
			name:       "none from upstream spec with PNI enabled",
			prevPolicy: string(armcontainerservice.NetworkPolicyNone),
			pniEnabled: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clusterSpec := &v32.ClusterSpec{
				ClusterSpecBase: v32.ClusterSpecBase{
					EnableNetworkPolicy: ptr.To(tt.pniEnabled),
				},
			}

			var prevCluster *v3.Cluster

			if tt.networkPolicy != "" {
				clusterSpec.AKSConfig = &aksv1.AKSClusterConfigSpec{
					NetworkPolicy: ptr.To(tt.networkPolicy),
				}
			} else if tt.prevPolicy != "" {
				prevCluster = &v3.Cluster{
					Status: v32.ClusterStatus{
						AKSStatus: v32.AKSStatus{
							UpstreamSpec: &aksv1.AKSClusterConfigSpec{
								NetworkPolicy: ptr.To(tt.prevPolicy),
							},
						},
					},
				}
			}

			err := validator.validateAKSNetworkPolicy(clusterSpec, prevCluster)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
