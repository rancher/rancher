package operations

import (
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
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

func TestImportedAdapter_ComponentTLSSettingsFromNodeArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		component string
		want      ComponentTLSSettings
	}{
		{
			name: "no relevant arguments",
			args: []string{
				"server",
				"--kube-apiserver-arg", "secure-port=6444",
				"--kube-controller-manager-arg", "feature-gates=Example=true",
			},
			component: KubeControllerManagerProbeName,
		},
		{
			name: "split outer form",
			args: []string{
				"--kube-controller-manager-arg", "secure-port=10261",
				"--kube-controller-manager-arg", "tls-cert-file=/custom/kcm.crt",
				"--kube-controller-manager-arg", "tls-private-key-file=/custom/kcm.key",
			},
			component: KubeControllerManagerProbeName,
			want: ComponentTLSSettings{
				SecurePort:        "10261",
				TLSCertFile:       "/custom/kcm.crt",
				TLSPrivateKeyFile: "/custom/kcm.key",
			},
		},
		{
			name: "combined outer form",
			args: []string{
				"--kube-controller-manager-arg=secure-port=10261",
				"--kube-controller-manager-arg=tls-cert-file=/custom/kcm.crt",
				"--kube-controller-manager-arg=tls-private-key-file=/custom/kcm.key",
			},
			component: KubeControllerManagerProbeName,
			want: ComponentTLSSettings{
				SecurePort:        "10261",
				TLSCertFile:       "/custom/kcm.crt",
				TLSPrivateKeyFile: "/custom/kcm.key",
			},
		},
		{
			name: "custom secure port",
			args: []string{
				"--kube-scheduler-arg", "secure-port=10262",
			},
			component: KubeSchedulerProbeName,
			want:      ComponentTLSSettings{SecurePort: "10262"},
		},
		{
			name: "complete custom TLS pair",
			args: []string{
				"--kube-scheduler-arg", "tls-cert-file=/custom/ks.crt",
				"--kube-scheduler-arg", "tls-private-key-file=/custom/ks.key",
			},
			component: KubeSchedulerProbeName,
			want: ComponentTLSSettings{
				TLSCertFile:       "/custom/ks.crt",
				TLSPrivateKeyFile: "/custom/ks.key",
			},
		},
		{
			name: "incomplete TLS pair",
			args: []string{
				"--kube-controller-manager-arg", "tls-cert-file=/custom/kcm.crt",
			},
			component: KubeControllerManagerProbeName,
			want:      ComponentTLSSettings{TLSCertFile: "/custom/kcm.crt"},
		},
		{
			name: "cert-dir is ignored",
			args: []string{
				"--kube-controller-manager-arg", "cert-dir=/custom",
			},
			component: KubeControllerManagerProbeName,
		},
		{
			name: "controller and scheduler select their own outer arguments",
			args: []string{
				"--kube-controller-manager-arg", "secure-port=10261",
				"--kube-scheduler-arg", "secure-port=10262",
			},
			component: KubeSchedulerProbeName,
			want:      ComponentTLSSettings{SecurePort: "10262"},
		},
		{
			name: "unknown component returns empty",
			args: []string{
				"--kube-controller-manager-arg", "secure-port=10261",
			},
			component: "unknown-component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := componentTLSSettingsFromNodeArgs(tt.args, tt.component)
			assert.Equal(t, tt.want, got)
			if tt.name == "incomplete TLS pair" {
				assert.False(t, got.HasCompleteTLSConfig())
			}
			if tt.name == "complete custom TLS pair" {
				assert.True(t, got.HasCompleteTLSConfig())
			}
		})
	}
}
