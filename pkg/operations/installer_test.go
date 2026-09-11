package operations

import (
	"strings"
	"testing"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	provcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
)

// --- InstallInstruction ---------------------------------------------------------------------

func TestCAPRAdapterInstallInstruction(t *testing.T) {
	t.Parallel()

	t.Run("installs the controlplane's kubernetes version without starting it", func(t *testing.T) {
		t.Parallel()

		a := &CAPRAdapter{
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					KubernetesVersion: "v1.33.0+rke2r1",
					AgentEnvVars: []rkev1.EnvVar{
						{Name: "HTTP_PROXY", Value: "http://proxy:3128"},
						{Name: "IGNORED", Value: ""},
					},
				},
			},
		}

		install, ok := a.InstallInstruction(nil)
		require.True(t, ok)

		assert.Equal(t, "install", install.Name)
		assert.Equal(t, "sh", install.Command)
		assert.Equal(t, []string{"-c", "run.sh"}, install.Args)

		// The image tag is the Kubernetes version with "+" swapped for "-", since "+" is not legal
		// in an image tag.
		assert.True(t, strings.HasSuffix(install.Image, "rke2:v1.33.0-rke2r1"),
			"image %q should be tagged with the kubernetes version", install.Image)

		assert.Contains(t, install.Env, "INSTALL_RKE2_SKIP_START=true")
		assert.Contains(t, install.Env, "HTTP_PROXY=http://proxy:3128")
		// An agent env var with no value would install an empty environment entry.
		for _, e := range install.Env {
			assert.NotEqual(t, "IGNORED=", e)
		}
	})

	t.Run("uses the k3s runtime for a k3s version", func(t *testing.T) {
		t.Parallel()

		a := &CAPRAdapter{
			controlPlane: &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{KubernetesVersion: "v1.33.0+k3s1"},
			},
		}

		install, ok := a.InstallInstruction(nil)
		require.True(t, ok)

		assert.True(t, strings.HasSuffix(install.Image, "k3s:v1.33.0-k3s1"), "image = %q", install.Image)
		assert.Contains(t, install.Env, "INSTALL_K3S_SKIP_START=true")
	})

	t.Run("reports unsupported when no version is configured", func(t *testing.T) {
		t.Parallel()

		a := &CAPRAdapter{controlPlane: &rkev1.RKEControlPlane{}}

		_, ok := a.InstallInstruction(nil)
		assert.False(t, ok)
	})
}

func TestCAPRKE2AdapterInstallInstruction(t *testing.T) {
	t.Parallel()

	t.Run("installs the RKE2ControlPlane's version", func(t *testing.T) {
		t.Parallel()

		a := &CAPRKE2Adapter{
			controlPlane: &controlplanev1beta2.RKE2ControlPlane{
				Spec: controlplanev1beta2.RKE2ControlPlaneSpec{Version: "v1.33.0+rke2r1"},
			},
		}

		install, ok := a.InstallInstruction(nil)
		require.True(t, ok)

		assert.True(t, strings.HasSuffix(install.Image, "rke2:v1.33.0-rke2r1"), "image = %q", install.Image)
		assert.Contains(t, install.Env, "INSTALL_RKE2_SKIP_START=true")
	})

	t.Run("reports unsupported when no version is set", func(t *testing.T) {
		t.Parallel()

		a := &CAPRKE2Adapter{controlPlane: &controlplanev1beta2.RKE2ControlPlane{}}

		_, ok := a.InstallInstruction(nil)
		assert.False(t, ok)
	})
}

func TestImportedAdapterInstallInstruction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cluster     *mgmtv3.Cluster
		wantVersion string
		wantOK      bool
	}{
		{
			name: "prefers the desired rke2 version",
			cluster: &mgmtv3.Cluster{
				Spec:   mgmtv3.ClusterSpec{Rke2Config: &mgmtv3.Rke2Config{Version: "v1.32.5+rke2r1"}},
				Status: mgmtv3.ClusterStatus{Driver: mgmtv3.ClusterDriverRke2, Version: &version.Info{GitVersion: "v1.34.1+rke2r1"}},
			},
			wantVersion: "rke2:v1.32.5-rke2r1",
			wantOK:      true,
		},
		{
			name: "prefers the desired k3s version",
			cluster: &mgmtv3.Cluster{
				Spec:   mgmtv3.ClusterSpec{K3sConfig: &mgmtv3.K3sConfig{Version: "v1.32.5+k3s1"}},
				Status: mgmtv3.ClusterStatus{Driver: mgmtv3.ClusterDriverK3s, Version: &version.Info{GitVersion: "v1.34.1+k3s1"}},
			},
			wantVersion: "k3s:v1.32.5-k3s1",
			wantOK:      true,
		},
		{
			// A cluster whose version is not managed has no desired version, so the reported one is
			// the only thing to install — a reinstall of what is already there.
			name: "falls back to the reported version",
			cluster: &mgmtv3.Cluster{
				Status: mgmtv3.ClusterStatus{Driver: mgmtv3.ClusterDriverRke2, Version: &version.Info{GitVersion: "v1.34.1+rke2r1"}},
			},
			wantVersion: "rke2:v1.34.1-rke2r1",
			wantOK:      true,
		},
		{
			// The distro config for the other driver must not be consulted.
			name: "ignores the config that does not match the driver",
			cluster: &mgmtv3.Cluster{
				Spec:   mgmtv3.ClusterSpec{K3sConfig: &mgmtv3.K3sConfig{Version: "v1.32.5+k3s1"}},
				Status: mgmtv3.ClusterStatus{Driver: mgmtv3.ClusterDriverRke2, Version: &version.Info{GitVersion: "v1.34.1+rke2r1"}},
			},
			wantVersion: "rke2:v1.34.1-rke2r1",
			wantOK:      true,
		},
		{
			name:    "reports unsupported when no version is known at all",
			cluster: &mgmtv3.Cluster{},
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := &ImportedAdapter{cluster: tt.cluster}

			install, ok := a.InstallInstruction(nil)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				return
			}

			assert.True(t, strings.HasSuffix(install.Image, tt.wantVersion), "image = %q, want suffix %q", install.Image, tt.wantVersion)
		})
	}
}

// --- WaitForRestoreTarget -------------------------------------------------------------------

// caprAdapterForWait builds a CAPRAdapter whose provisioning-cluster cache serves cluster, and whose
// RKEControlPlane carries renderedSpec as its cluster-spec annotation.
func caprAdapterForWait(t *testing.T, cluster *provv1.Cluster, renderedSpec *provv1.ClusterSpec) *CAPRAdapter {
	t.Helper()

	ctrl := gomock.NewController(t)
	clusterCache := ctrlfake.NewMockCacheInterface[*provv1.Cluster](ctrl)
	clusters := ctrlfake.NewMockControllerInterface[*provv1.Cluster, *provv1.ClusterList](ctrl)
	clusters.EXPECT().Cache().Return(clusterCache).AnyTimes()
	clusterCache.EXPECT().Get(cluster.Namespace, cluster.Name).Return(cluster, nil).AnyTimes()

	annotations := map[string]string{}
	if renderedSpec != nil {
		payload, err := snapshotutil.CompressInterface(renderedSpec)
		require.NoError(t, err)
		annotations[capr.ClusterSpecAnnotation] = payload
	}

	return &CAPRAdapter{
		controlPlane: &rkev1.RKEControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   cluster.Namespace,
				Name:        cluster.Name,
				Annotations: annotations,
			},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Provisioning: stubProvisioningInterface{clusters: clusters},
			},
		},
	}
}

// stubProvisioningInterface serves only Cluster(); every other accessor is unused by the adapter.
type stubProvisioningInterface struct {
	provcontrollers.Interface

	clusters provcontrollers.ClusterController
}

func (s stubProvisioningInterface) Cluster() provcontrollers.ClusterController {
	return s.clusters
}

func TestCAPRAdapterWaitForRestoreTarget(t *testing.T) {
	t.Parallel()

	liveCluster := func(version string) *provv1.Cluster {
		return &provv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "example"},
			Spec: provv1.ClusterSpec{
				KubernetesVersion: version,
				RKEConfig:         &provv1.RKEConfig{},
			},
		}
	}

	t.Run("settled when the rendered spec matches the live one", func(t *testing.T) {
		t.Parallel()

		cluster := liveCluster("v1.33.0+rke2r1")
		a := caprAdapterForWait(t, cluster, cluster.Spec.DeepCopy())

		settled, err := a.WaitForRestoreTarget()
		require.NoError(t, err)
		assert.True(t, settled)
	})

	t.Run("not settled when the controlplane was rendered from an older spec", func(t *testing.T) {
		t.Parallel()

		// The restore has written v1.33.0 onto the provisioning cluster, but the controlplane still
		// carries the spec it was rendered from. Acting now would install v1.34.1.
		stale := liveCluster("v1.34.1+rke2r1").Spec.DeepCopy()
		a := caprAdapterForWait(t, liveCluster("v1.33.0+rke2r1"), stale)

		settled, err := a.WaitForRestoreTarget()
		require.NoError(t, err)
		assert.False(t, settled)
	})

	t.Run("not settled when the controlplane carries no rendered spec", func(t *testing.T) {
		t.Parallel()

		a := caprAdapterForWait(t, liveCluster("v1.33.0+rke2r1"), nil)

		settled, err := a.WaitForRestoreTarget()
		require.NoError(t, err)
		assert.False(t, settled)
	})

	t.Run("operation fields do not hold the wait open", func(t *testing.T) {
		t.Parallel()

		// provisioningcluster strips the four operation fields before rendering, so a live cluster
		// carrying an in-flight restore still counts as settled. Without this the wait could never
		// finish: the restore itself sets ETCDSnapshotRestore.
		cluster := liveCluster("v1.33.0+rke2r1")
		rendered := cluster.Spec.DeepCopy()
		cluster.Spec.RKEConfig.ETCDSnapshotRestore = &rkev1.ETCDSnapshotRestore{Name: "snapshot-1", Generation: 2}
		cluster.Spec.RKEConfig.RotateCertificates = &rkev1.RotateCertificates{Generation: 3}

		a := caprAdapterForWait(t, cluster, rendered)

		settled, err := a.WaitForRestoreTarget()
		require.NoError(t, err)
		assert.True(t, settled)
	})

	t.Run("errors on an undecodable rendered spec", func(t *testing.T) {
		t.Parallel()

		a := caprAdapterForWait(t, liveCluster("v1.33.0+rke2r1"), nil)
		a.controlPlane.Annotations[capr.ClusterSpecAnnotation] = "not-base64-or-gzip"

		_, err := a.WaitForRestoreTarget()
		require.Error(t, err)
	})
}

func TestCAPRKE2AndImportedWaitForRestoreTarget(t *testing.T) {
	t.Parallel()

	// Neither renders anything off its restore target, so neither ever waits.
	settled, err := (&CAPRKE2Adapter{}).WaitForRestoreTarget()
	require.NoError(t, err)
	assert.True(t, settled)

	settled, err = (&ImportedAdapter{}).WaitForRestoreTarget()
	require.NoError(t, err)
	assert.True(t, settled)
}
