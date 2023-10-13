package podsecuritypolicy

import (
	"fmt"
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"
)

func newClusterLister(kubernetesVersion string) *fakes.ClusterListerMock {
	return &fakes.ClusterListerMock{
		GetFunc: func(namespace, name string) (*v3.Cluster, error) {
			if name == "invalid" {
				return nil, fmt.Errorf("invalid cluster: %s", name)
			} else if name == "not ready" {
				return &v3.Cluster{Status: v3.ClusterStatus{}}, nil
			} else {
				return &v3.Cluster{Status: v3.ClusterStatus{Version: &version.Info{GitVersion: kubernetesVersion}}}, nil
			}
		},
	}
}

func TestCheckClusterVersionFailsForVersionsThatCannotBeParsed(t *testing.T) {
	t.Parallel()
	tests := []string{"", "⌘⌘⌘", "v1.2", "v1.24", "v1.24.a", "1.24", "1.24.a"}
	for _, v := range tests {
		v := v
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			clusterLister := newClusterLister(v)
			err := checkClusterVersion("test", clusterLister)
			require.Error(t, err)
			require.NotErrorIs(t, err, errVersionIncompatible)
		})
	}
}

func TestCheckClusterVersionInspectsValidVersionsForCompatibilityWithPSP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		version string
		wantErr bool
	}{
		// regular version strings
		{
			version: "1.24.9",
			wantErr: false,
		},
		{
			version: "v1.24.9",
			wantErr: false,
		},
		{
			version: "1.25.9",
			wantErr: true,
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
		// rke1 version strings
		{
			version: "v1.24.9-rancher1-1",
			wantErr: false,
		},
		{
			version: "v1.25.9-rancher1-1",
			wantErr: true,
		},
		{
			version: "v1.26.9-rancher1-1",
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
		tt := tt
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()
			clusterLister := newClusterLister(tt.version)
			err := checkClusterVersion("test", clusterLister)
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, errVersionIncompatible)
			}
			if !tt.wantErr {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckClusterVersionFailsWhenItCannotFetchVersion(t *testing.T) {
	t.Parallel()
	t.Run("version check fails when it can't get cluster", func(t *testing.T) {
		t.Parallel()
		clusterLister := newClusterLister("")
		err := checkClusterVersion("invalid", clusterLister)
		require.Error(t, err)
		require.NotErrorIs(t, err, errVersionIncompatible)
	})

	t.Run("version check fails when the version is not yet known", func(t *testing.T) {
		t.Parallel()
		clusterLister := newClusterLister("")
		err := checkClusterVersion("not ready", clusterLister)
		require.Error(t, err)
		require.NotErrorIs(t, err, errVersionIncompatible)
	})
}
