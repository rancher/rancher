package operations

import (
	"errors"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/importedclusterversionmanagement"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// --- RuntimeCommand / ServerUnit ------------------------------------------------------------

func TestImportedAdapter_RuntimeCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider string
		want     string
	}{
		{"rke2 provider", "rke2", "rke2"},
		{"k3s provider", "k3s", "k3s"},
		{"empty provider defaults to k3s", "", "k3s"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &ImportedAdapter{
				cluster: &mgmtv3.Cluster{
					Status: mgmtv3.ClusterStatus{
						Provider: tc.provider,
					},
				},
			}
			got := a.RuntimeCommand()
			assert.Equal(t, tc.want, got, "RuntimeCommand mismatch for provider=%q", tc.provider)
		})
	}
}

func TestImportedAdapter_ServerUnit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider string
		want     string
	}{
		{"rke2 provider", "rke2", "rke2-server"},
		{"k3s provider", "k3s", "k3s"},
		{"empty provider defaults to k3s", "", "k3s"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &ImportedAdapter{
				cluster: &mgmtv3.Cluster{
					Status: mgmtv3.ClusterStatus{
						Provider: tc.provider,
					},
				},
			}
			got := a.ServerUnit()
			assert.Equal(t, tc.want, got, "ServerUnit mismatch for provider=%q", tc.provider)
		})
	}
}

// --- WaitForRegister ------------------------------------------------------------------------

func newImportedMachinePlanSecret(name, machineName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "c-mine",
			UID:       types.UID(name + "-uid"),
			Labels: map[string]string{
				planv1alpha1.ClusterLifecycleGroupLabel: "management.cattle.io",
				planv1alpha1.ClusterLifecycleKindLabel:  "Cluster",
				planv1alpha1.ClusterLifecycleNameLabel:  "c-mine",
				planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  machineName,
			},
		},
		Type: capr.SecretTypeMachinePlan,
	}
}

func newMgmtNode(name string) *mgmtv3.Node {
	return &mgmtv3.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "c-mine",
		},
	}
}

func TestImportedAdapter_WaitForRegister_Perfect1to1(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secrets := []*corev1.Secret{
		newImportedMachinePlanSecret("secret-a", "node-a"),
		newImportedMachinePlanSecret("secret-b", "node-b"),
	}
	nodes := []*mgmtv3.Node{
		newMgmtNode("node-a"),
		newMgmtNode("node-b"),
	}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	nodeCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nodes, nil)

	// Create stub core and mgmt interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
				Mgmt: stubMgmt,
			},
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.True(t, ok, "perfect 1:1 match should return true")
}

func TestImportedAdapter_WaitForRegister_CountMismatch(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secrets := []*corev1.Secret{newImportedMachinePlanSecret("secret-a", "node-a")}
	nodes := []*mgmtv3.Node{
		newMgmtNode("node-a"),
		newMgmtNode("node-b"),
	}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	nodeCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nodes, nil)

	// Create stub core and mgmt interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
				Mgmt: stubMgmt,
			},
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "count mismatch should return false")
}

func TestImportedAdapter_WaitForRegister_DuplicateSecrets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	// Two secrets pointing to the same node.
	secrets := []*corev1.Secret{
		newImportedMachinePlanSecret("secret-a", "node-a"),
		newImportedMachinePlanSecret("secret-b", "node-a"),
	}
	nodes := []*mgmtv3.Node{
		newMgmtNode("node-a"),
		newMgmtNode("node-b"),
	}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	nodeCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nodes, nil)

	// Create stub core and mgmt interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
				Mgmt: stubMgmt,
			},
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "duplicate secrets (same node) should return false")
}

func TestImportedAdapter_WaitForRegister_MissingMachineNameLabel(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secretNoLabel := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-a",
			Namespace: "c-mine",
			Labels: map[string]string{
				capr.ClusterNameLabel: "c-mine",
				// No MachineNameLabel
			},
		},
		Type: capr.SecretTypeMachinePlan,
	}
	secrets := []*corev1.Secret{secretNoLabel}
	nodes := []*mgmtv3.Node{newMgmtNode("node-a")}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	nodeCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nodes, nil)

	// Create stub core and mgmt interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
				Mgmt: stubMgmt,
			},
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "secret without machine-name label should return false")
}

func TestImportedAdapter_WaitForRegister_NilLabels(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secretNilLabels := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-a",
			Namespace: "c-mine",
			Labels:    nil,
		},
		Type: capr.SecretTypeMachinePlan,
	}
	secrets := []*corev1.Secret{secretNilLabels}
	nodes := []*mgmtv3.Node{newMgmtNode("node-a")}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	nodeCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nodes, nil)

	// Create stub core and mgmt interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
				Mgmt: stubMgmt,
			},
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "secret with nil labels should return false")
}

func TestImportedAdapter_WaitForRegister_SecretPointsToUnexpectedNode(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	// Secret points to "node-phantom" which doesn't exist in the node list.
	secrets := []*corev1.Secret{newImportedMachinePlanSecret("secret-a", "node-phantom")}
	nodes := []*mgmtv3.Node{newMgmtNode("node-a")}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	nodeCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(nodes, nil)

	// Create stub core and mgmt interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
				Mgmt: stubMgmt,
			},
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "secret pointing to unexpected node should return false")
}

// newPauseAdapter wires an ImportedAdapter over a stub mgmt client holding the given cluster.
func newPauseAdapter(cluster *mgmtv3.Cluster) (*ImportedAdapter, *stubClusterController) {
	clusters := &stubClusterController{
		clusters: map[string]*mgmtv3.Cluster{cluster.Name: cluster},
	}
	return &ImportedAdapter{
		cluster: cluster,
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Mgmt: &stubMgmtInterface{clusters: clusters},
			},
		},
	}, clusters
}

func newPauseCluster(annotations map[string]string) *mgmtv3.Cluster {
	return &mgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-mine", Annotations: annotations},
	}
}

func TestImportedAdapter_PauseCluster(t *testing.T) {
	t.Parallel()

	const anno = importedclusterversionmanagement.VersionManagementPausedAnno

	tests := []struct {
		name        string
		annotations map[string]string
		pause       bool
		wantUpdate  bool
		wantPaused  bool
	}{
		{
			name:       "pausing an unannotated cluster",
			pause:      true,
			wantUpdate: true,
			wantPaused: true,
		},
		{
			name:        "pausing preserves unrelated annotations",
			annotations: map[string]string{importedclusterversionmanagement.VersionManagementAnno: "true"},
			pause:       true,
			wantUpdate:  true,
			wantPaused:  true,
		},
		{
			name:        "pausing an already paused cluster does not write",
			annotations: map[string]string{anno: "true"},
			pause:       true,
			wantUpdate:  false,
			wantPaused:  true,
		},
		{
			name:        "unpausing removes the annotation",
			annotations: map[string]string{anno: "true"},
			pause:       false,
			wantUpdate:  true,
			wantPaused:  false,
		},
		{
			name:        "unpausing an unpaused cluster does not write",
			annotations: nil,
			pause:       false,
			wantUpdate:  false,
			wantPaused:  false,
		},
		{
			// The annotation is only ever written as "true", but a stray value must still be cleaned
			// up on unpause rather than left behind for the upgrade handler to interpret.
			name:        "unpausing removes a non-true value",
			annotations: map[string]string{anno: "false"},
			pause:       false,
			wantUpdate:  true,
			wantPaused:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			adapter, clusters := newPauseAdapter(newPauseCluster(tt.annotations))

			require.NoError(t, adapter.PauseCluster(tt.pause))

			if tt.wantUpdate {
				require.Len(t, clusters.updates, 1, "expected exactly one write")
			} else {
				assert.Empty(t, clusters.updates, "no-op must not write the cluster")
			}

			latest, err := clusters.Get("c-mine", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.wantPaused, importedclusterversionmanagement.Paused(latest))

			if !tt.wantPaused && tt.wantUpdate {
				assert.NotContains(t, latest.Annotations, anno, "unpausing must remove the annotation, not blank it")
			}
			if tt.annotations[importedclusterversionmanagement.VersionManagementAnno] != "" {
				assert.Equal(t, "true", latest.Annotations[importedclusterversionmanagement.VersionManagementAnno],
					"unrelated annotations must survive")
			}
		})
	}
}

func TestImportedAdapter_PauseCluster_GetError(t *testing.T) {
	t.Parallel()

	adapter, clusters := newPauseAdapter(newPauseCluster(nil))
	clusters.getErr = errors.New("apiserver is down")

	// The restore must not proceed believing it paused the cluster.
	assert.ErrorContains(t, adapter.PauseCluster(true), "apiserver is down")
	assert.Empty(t, clusters.updates)
}
