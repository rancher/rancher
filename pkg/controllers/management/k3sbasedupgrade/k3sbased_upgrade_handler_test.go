package k3sbasedupgrade

import (
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestOnClusterChange_Paused covers the pause gate. The handler is deliberately zero-valued: every
// path past the gate dereferences h.manager or h.clusterClient, so these cases only pass because the
// gate returns before any of that — which is the point, since an etcd snapshot restore needs version
// management to render nothing at all while it rewrites the cluster's version.
func TestOnClusterChange_Paused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
	}{
		{
			name: "paused with version management enabled",
			annotations: map[string]string{
				importedclusterversionmanagement.VersionManagementAnno:       "true",
				importedclusterversionmanagement.VersionManagementPausedAnno: "true",
			},
		},
		{
			// Version management being disabled is not a substitute: that branch still reaches into
			// the downstream cluster to delete plans, which a restore has no business doing.
			name: "paused with version management disabled",
			annotations: map[string]string{
				importedclusterversionmanagement.VersionManagementAnno:       "false",
				importedclusterversionmanagement.VersionManagementPausedAnno: "true",
			},
		},
		{
			name: "paused with no version management annotation",
			annotations: map[string]string{
				importedclusterversionmanagement.VersionManagementPausedAnno: "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := &handler{}
			cluster := &mgmtv3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-mine", Annotations: tt.annotations},
				Status:     mgmtv3.ClusterStatus{Driver: mgmtv3.ClusterDriverRke2},
				Spec: mgmtv3.ClusterSpec{
					Rke2Config: &mgmtv3.Rke2Config{},
				},
			}
			cluster.Spec.Rke2Config.Version = "v1.33.0+rke2r1"

			got, err := h.onClusterChange("", cluster)
			assert.NoError(t, err)
			assert.Equal(t, cluster, got, "a paused cluster must come back untouched")
		})
	}
}

// TestOnClusterChange_NotPaused_IgnoresOtherDrivers pins the gate's position: it is evaluated after
// the driver check, so a cluster this controller does not manage is still short-circuited first.
func TestOnClusterChange_NotPaused_IgnoresOtherDrivers(t *testing.T) {
	t.Parallel()

	h := &handler{}
	cluster := &mgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-other"},
		Status:     mgmtv3.ClusterStatus{Driver: mgmtv3.ClusterDriverImported},
	}

	got, err := h.onClusterChange("", cluster)
	assert.NoError(t, err)
	assert.Equal(t, cluster, got)
}
