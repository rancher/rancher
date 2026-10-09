package operations

import (
	"maps"
	"testing"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestApplyWhitelistChange(t *testing.T) {
	const whitelisted = opv1alpha1.WhitelistedAnnotation
	const restores = opv1alpha1.ETCDSnapshotRestoreResource

	for name, tc := range map[string]struct {
		annotations map[string]string
		change      WhitelistChange
		want        map[string]string
		changed     bool
	}{
		"restores on a nil map":         {annotations: nil, change: WhitelistRestores, want: map[string]string{whitelisted: restores}, changed: true},
		"restores keeps other entries":  {annotations: map[string]string{whitelisted: "a.example.io", "x": "y"}, change: WhitelistRestores, want: map[string]string{whitelisted: "a.example.io," + restores, "x": "y"}, changed: true},
		"restores already whitelisted":  {annotations: map[string]string{whitelisted: restores}, change: WhitelistRestores, want: map[string]string{whitelisted: restores}},
		"cleared removes it":            {annotations: map[string]string{whitelisted: restores, "x": "y"}, change: WhitelistCleared, want: map[string]string{"x": "y"}, changed: true},
		"cleared removes an empty one":  {annotations: map[string]string{whitelisted: ""}, change: WhitelistCleared, want: map[string]string{}, changed: true},
		"cleared with nothing to clear": {annotations: map[string]string{"x": "y"}, change: WhitelistCleared, want: map[string]string{"x": "y"}},
		"cleared on a nil map":          {annotations: nil, change: WhitelistCleared, want: nil},
		"unchanged leaves it":           {annotations: map[string]string{whitelisted: restores}, change: WhitelistUnchanged, want: map[string]string{whitelisted: restores}},
		"unchanged on a nil map":        {annotations: nil, change: WhitelistUnchanged, want: nil},
	} {
		t.Run(name, func(t *testing.T) {
			got, changed := ApplyWhitelistChange(tc.annotations, tc.change)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.changed, changed)
		})
	}
}

// The CAPR and CAPRKE2 adapters pause the CAPI Cluster, and write the whitelist to the objects operations
// and the UI key off: the management Cluster for both, and for CAPR the provisioning Cluster and the
// RKEControlPlane too. Each object is written only when its part changes, and a call that changes nothing
// (re-asserting a pause, or unpausing for a restart with the whitelist unchanged) succeeds without writing.
func TestCAPIAdapters_PauseCluster(t *testing.T) {
	const whitelisted = opv1alpha1.WhitelistedAnnotation
	const restores = opv1alpha1.ETCDSnapshotRestoreResource

	// whitelistedObject reads back the whitelist an object carries and how many times it was written.
	type whitelistedObject struct {
		name    string
		writes  func() int
		current func() map[string]string
	}

	type fixture struct {
		adapter     Adapter
		capi        *stubCAPIClusterController
		whitelisted []whitelistedObject
	}

	mgmtCluster := func(annotations map[string]string) (*stubClusterController, whitelistedObject) {
		clusters := &stubClusterController{clusters: map[string]*mgmtv3.Cluster{
			"c-abc": {Name: "c-abc", Annotations: annotations},
		}}
		return clusters, whitelistedObject{
			name:    "management Cluster",
			writes:  func() int { return len(clusters.updates) },
			current: func() map[string]string { return clusters.clusters["c-abc"].Annotations },
		}
	}

	adapters := map[string]func(capiClusters *stubCAPIClusterController, annotations map[string]string) fixture{
		"CAPR": func(capiClusters *stubCAPIClusterController, annotations map[string]string) fixture {
			mgmt, mgmtObject := mgmtCluster(maps.Clone(annotations))
			prov := &stubProvisioningClusterController{cluster: &provv1.Cluster{
				Namespace: "fleet-default", Name: "c", Annotations: maps.Clone(annotations),
				Status: provv1.ClusterStatus{ClusterName: "c-abc"},
			}}
			controlPlanes := &stubRKEControlPlaneController{controlPlane: &rkev1.RKEControlPlane{
				Namespace: "fleet-default", Name: "c", Annotations: maps.Clone(annotations),
			}}
			clients := &wrangler.CAPIContext{
				Context: &wrangler.Context{
					Mgmt:         &stubMgmtInterface{clusters: mgmt},
					Provisioning: stubProvisioningInterface{clusters: prov},
					RKE:          &stubRKEInterface{controlPlanes: controlPlanes},
				},
				CAPI: &stubCAPIInterface{clusters: capiClusters},
			}
			return fixture{
				adapter: &CAPRAdapter{clients: clients, controlPlane: &rkev1.RKEControlPlane{Namespace: "fleet-default", Name: "c"}},
				capi:    capiClusters,
				whitelisted: []whitelistedObject{
					mgmtObject,
					{
						name:    "provisioning Cluster",
						writes:  func() int { return len(prov.updates) },
						current: func() map[string]string { return prov.cluster.Annotations },
					},
					{
						name:    "RKEControlPlane",
						writes:  func() int { return len(controlPlanes.updates) },
						current: func() map[string]string { return controlPlanes.controlPlane.Annotations },
					},
				},
			}
		},
		"CAPRKE2": func(capiClusters *stubCAPIClusterController, annotations map[string]string) fixture {
			mgmt, mgmtObject := mgmtCluster(maps.Clone(annotations))
			clients := &wrangler.CAPIContext{
				Context: &wrangler.Context{Mgmt: &stubMgmtInterface{clusters: mgmt}},
				CAPI:    &stubCAPIInterface{clusters: capiClusters},
			}
			return fixture{
				adapter: &CAPRKE2Adapter{
					clients:         clients,
					cluster:         &capi.Cluster{Namespace: "fleet-default", Name: "c"},
					controlPlane:    &controlplanev1beta2.RKE2ControlPlane{Namespace: "fleet-default", Name: "c"},
					mgmtClusterName: "c-abc",
				},
				capi:        capiClusters,
				whitelisted: []whitelistedObject{mgmtObject},
			}
		},
	}

	tests := []struct {
		name        string
		paused      *bool
		annotations map[string]string
		pause       bool
		whitelist   WhitelistChange

		wantPauseWrite     bool
		wantWhitelistWrite bool
		wantWhitelist      string
	}{
		{
			name:               "pausing whitelists restores",
			pause:              true,
			whitelist:          WhitelistRestores,
			wantPauseWrite:     true,
			wantWhitelistWrite: true,
			wantWhitelist:      restores,
		},
		{
			name:               "whitelisting an already paused cluster writes only the whitelist",
			paused:             new(true),
			pause:              true,
			whitelist:          WhitelistRestores,
			wantWhitelistWrite: true,
			wantWhitelist:      restores,
		},
		{
			// Re-asserted on every reconcile of a rotation's Rotate step.
			name:          "re-asserting the pause on a paused, whitelisted cluster writes nothing",
			paused:        new(true),
			annotations:   map[string]string{whitelisted: restores},
			pause:         true,
			whitelist:     WhitelistRestores,
			wantWhitelist: restores,
		},
		{
			name:               "unpausing clears the whitelist",
			paused:             new(true),
			annotations:        map[string]string{whitelisted: restores},
			pause:              false,
			whitelist:          WhitelistCleared,
			wantPauseWrite:     true,
			wantWhitelistWrite: true,
		},
		{
			// The restore unpauses for its restart and keeps the whitelist until it succeeds.
			name:           "unpausing can leave the whitelist",
			paused:         new(true),
			annotations:    map[string]string{whitelisted: restores},
			pause:          false,
			whitelist:      WhitelistUnchanged,
			wantPauseWrite: true,
			wantWhitelist:  restores,
		},
		{
			name:      "an unpaused cluster without a whitelist writes nothing",
			paused:    new(false),
			pause:     false,
			whitelist: WhitelistCleared,
		},
	}

	for kind, newFixture := range adapters {
		for _, tt := range tests {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				capiClusters := &stubCAPIClusterController{cluster: &capi.Cluster{
					Namespace: "fleet-default", Name: "c",
					Spec: capi.ClusterSpec{Paused: tt.paused},
				}}
				fx := newFixture(capiClusters, tt.annotations)

				require.NoError(t, fx.adapter.PauseCluster(tt.pause, tt.whitelist))

				if tt.wantPauseWrite {
					require.Len(t, fx.capi.updates, 1, "the CAPI Cluster's pause is written once")
					assert.Equal(t, new(tt.pause), fx.capi.updates[0].Spec.Paused)
				} else {
					assert.Empty(t, fx.capi.updates, "the pause is already as asked, so the CAPI Cluster isn't written")
				}

				for _, object := range fx.whitelisted {
					if tt.wantWhitelistWrite {
						assert.Equal(t, 1, object.writes(), "%s: the whitelist is written once", object.name)
					} else {
						assert.Zero(t, object.writes(), "%s: the whitelist is already as asked, so it isn't written", object.name)
					}
					if tt.wantWhitelist == "" {
						assert.NotContains(t, object.current(), whitelisted, "%s", object.name)
					} else {
						assert.Equal(t, tt.wantWhitelist, object.current()[whitelisted], "%s", object.name)
					}
				}
			})
		}
	}
}

func TestWhitelistProblem(t *testing.T) {
	const whitelisted = opv1alpha1.WhitelistedAnnotation
	const restores = opv1alpha1.ETCDSnapshotRestoreResource
	ref := &corev1.ObjectReference{APIVersion: "cluster.x-k8s.io/v1beta2", Kind: "Cluster", Namespace: "fleet-default", Name: "c"}

	for name, tc := range map[string]struct {
		annotations map[string]string
		resource    string
		want        string
	}{
		"no whitelist":              {resource: opv1alpha1.ETCDSnapshotSaveResource},
		"an empty whitelist":        {annotations: map[string]string{whitelisted: " "}, resource: opv1alpha1.ETCDSnapshotSaveResource},
		"a listed resource":         {annotations: map[string]string{whitelisted: restores}, resource: restores},
		"one of several, spaced":    {annotations: map[string]string{whitelisted: "a.example.io, " + restores}, resource: restores},
		"a save on a whitelist":     {annotations: map[string]string{whitelisted: restores}, resource: opv1alpha1.ETCDSnapshotSaveResource, want: "Cluster fleet-default/c only permits " + restores},
		"a rotation on a whitelist": {annotations: map[string]string{whitelisted: "a.example.io," + restores}, resource: opv1alpha1.CertificateRotationResource, want: "Cluster fleet-default/c only permits a.example.io, " + restores},
	} {
		t.Run(name, func(t *testing.T) {
			got := WhitelistProblem(ref, tc.annotations, tc.resource)
			if tc.want == "" {
				assert.Empty(t, got)
				return
			}
			assert.Contains(t, got, tc.want)
			assert.Contains(t, got, "requires an etcd snapshot restore")
		})
	}

	assert.Contains(t, WhitelistProblem(&corev1.ObjectReference{Kind: "Cluster", Name: "c-abc"}, map[string]string{whitelisted: restores}, opv1alpha1.ETCDSnapshotSaveResource),
		"Cluster c-abc only permits", "a cluster-scoped clusterRef is named without a namespace")
}
