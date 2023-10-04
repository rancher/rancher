package podsecuritypolicy

import (
	"fmt"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/version"
)

func newClusterListerWithVersion(kubernetesVersion string) *fakes.ClusterListerMock {
	return &fakes.ClusterListerMock{
		GetFunc: func(namespace, name string) (*mgmtv3.Cluster, error) {
			if name == "test" {
				cluster := mgmtv3.Cluster{
					Status: apimgmtv3.ClusterStatus{
						Version: &version.Info{
							GitVersion: kubernetesVersion,
						},
					},
				}
				return &cluster, nil
			}
			return nil, fmt.Errorf("invalid cluster: %s", name)
		},
	}
}

func TestCheckClusterVersion(t *testing.T) {

	tests := []*struct {
		version string
		wantErr bool
		setup   func()
	}{
		// tests for version string size
		{
			version: "",
			wantErr: true,
		},
		{
			version: "v1.24",
			wantErr: false,
		},
		{
			version: "v1.2",
			wantErr: true,
		},
		// rke1 version strings
		{
			version: "v1.24.9",
			wantErr: false,
		},
		{
			version: "v1.25.9",
			wantErr: true,
		},
		{
			version: "v1.26.9",
			wantErr: true,
		},
		// k3s version strings
		{
			version: "v1.24.9+k3s1",
			wantErr: false,
		},
		{
			version: "v1.25.9+k3s1",
			wantErr: true,
		},
		{
			version: "v1.26.9+k3s1",
			wantErr: true,
		},
		// rke2 version strings
		{
			version: "v1.24.9+rke2r1",
			wantErr: false,
		},
		{
			version: "v1.25.9+rke2r1",
			wantErr: true,
		},
		{
			version: "v1.26.9+rke2r1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			clusterLister := newClusterListerWithVersion(tt.version)
			println(tt.version)
			err := checkClusterVersion("test", clusterLister)
			if tt.wantErr {
				println(err.Error())
				assert.Error(t, err, "Expected checkClusterVersion to raise error.")
				return
			}
		})
	}

}
