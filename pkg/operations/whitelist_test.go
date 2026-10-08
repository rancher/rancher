package operations

import (
	"testing"

	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

// The CAPR and CAPRKE2 adapters pause the CAPI Cluster and write its whitelist in the same update, so a
// cluster can never be left paused by an operation without also recording that only a restore can
// repair it, and write nothing when there is nothing to change.
func TestCAPIAdapters_PauseCluster(t *testing.T) {
	const whitelisted = opv1alpha1.WhitelistedAnnotation
	const restores = opv1alpha1.ETCDSnapshotRestoreResource

	adapters := map[string]func(*wrangler.CAPIContext) Adapter{
		"CAPR": func(clients *wrangler.CAPIContext) Adapter {
			return &CAPRAdapter{clients: clients, controlPlane: &rkev1.RKEControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "c"}}}
		},
		"CAPRKE2": func(clients *wrangler.CAPIContext) Adapter {
			return &CAPRKE2Adapter{clients: clients, controlPlane: &controlplanev1beta2.RKE2ControlPlane{ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "c"}}}
		},
	}

	tests := []struct {
		name          string
		paused        *bool
		annotations   map[string]string
		pause         bool
		whitelist     WhitelistChange
		wantUpdate    bool
		wantWhitelist string
	}{
		{name: "pausing whitelists restores in the same write", pause: true, whitelist: WhitelistRestores, wantUpdate: true, wantWhitelist: restores},
		{name: "whitelisting an already paused cluster still writes", paused: ptr.To(true), pause: true, whitelist: WhitelistRestores, wantUpdate: true, wantWhitelist: restores},
		{name: "a paused, whitelisted cluster does not write", paused: ptr.To(true), annotations: map[string]string{whitelisted: restores}, pause: true, whitelist: WhitelistRestores, wantWhitelist: restores},
		{name: "unpausing clears the whitelist in the same write", paused: ptr.To(true), annotations: map[string]string{whitelisted: restores}, pause: false, whitelist: WhitelistCleared, wantUpdate: true},
		{name: "unpausing can leave the whitelist", paused: ptr.To(true), annotations: map[string]string{whitelisted: restores}, pause: false, whitelist: WhitelistUnchanged, wantUpdate: true, wantWhitelist: restores},
		{name: "an unpaused cluster without a whitelist does not write", paused: ptr.To(false), pause: false, whitelist: WhitelistCleared},
	}

	for kind, newAdapter := range adapters {
		for _, tt := range tests {
			t.Run(kind+"/"+tt.name, func(t *testing.T) {
				clusters := &stubCAPIClusterController{cluster: &capi.Cluster{
					ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "c", Annotations: tt.annotations},
					Spec:       capi.ClusterSpec{Paused: tt.paused},
				}}
				adapter := newAdapter(&wrangler.CAPIContext{CAPI: &stubCAPIInterface{clusters: clusters}})

				require.NoError(t, adapter.PauseCluster(tt.pause, tt.whitelist))

				if !tt.wantUpdate {
					assert.Empty(t, clusters.updates, "nothing to change, so nothing is written")
					return
				}
				require.Len(t, clusters.updates, 1, "the pause and the whitelist are written in one update")
				written := clusters.updates[0]
				assert.Equal(t, ptr.To(tt.pause), written.Spec.Paused)
				if tt.wantWhitelist == "" {
					assert.NotContains(t, written.Annotations, whitelisted)
				} else {
					assert.Equal(t, tt.wantWhitelist, written.Annotations[whitelisted])
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
