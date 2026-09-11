package operations

import (
	"encoding/json"
	"errors"
	"maps"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generic"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestImportedAdapter_RuntimeService(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider string
		secret   *corev1.Secret
		want     string
	}{
		{"rke2 control-plane", "rke2", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{capr.ControlPlaneRoleLabel: "true"}}}, "rke2-server"},
		{"rke2 etcd", "rke2", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{capr.EtcdRoleLabel: "true"}}}, "rke2-server"},
		{"rke2 worker-only", "rke2", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{capr.WorkerRoleLabel: "true"}}}, "rke2-agent"},
		{"k3s control-plane", "k3s", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{capr.ControlPlaneRoleLabel: "true"}}}, "k3s"},
		{"k3s worker-only", "k3s", &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{capr.WorkerRoleLabel: "true"}}}, "k3s-agent"},
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
			got := a.RuntimeService(tc.secret)
			assert.Equal(t, tc.want, got, "RuntimeService mismatch for %s", tc.name)
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
			name: "scheduler uses prefixed args",
			args: []string{
				"--kube-scheduler-arg", "--secure-port=10262",
				"--kube-scheduler-arg", "--tls-cert-file=/custom/ks.crt",
				"--kube-scheduler-arg", "--tls-private-key-file=/custom/ks.key",
			},
			component: KubeSchedulerProbeName,
			want: ComponentTLSSettings{
				SecurePort:        "10262",
				TLSCertFile:       "/custom/ks.crt",
				TLSPrivateKeyFile: "/custom/ks.key",
			},
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

// --- ComponentTLSSettings error handling --------------------------------

// fakeRESTMapper is a minimal RESTMapper implementation for testing
type fakeRESTMapper struct {
	meta.RESTMapper
}

func (f *fakeRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return &meta.RESTMapping{
		Resource:         mgmtv3.SchemeGroupVersion.WithResource("nodes"),
		GroupVersionKind: mgmtv3.SchemeGroupVersion.WithKind("Machine"),
		Scope:            meta.RESTScopeNamespace,
	}, nil
}

func TestImportedAdapter_ComponentTLSSettings_NodeNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "c-mine",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "node-a",
			},
		},
	}

	nodeCache.EXPECT().Get("c-mine", "node-a").Return(nil, apierrors.NewNotFound(
		mgmtv3.Resource("node"), "node-a"))

	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}
	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
			Status:     mgmtv3.ClusterStatus{Provider: "rke2"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Mgmt:       stubMgmt,
				RESTMapper: &fakeRESTMapper{},
			},
		},
	}

	_, err := adapter.ComponentTLSSettings(secret, KubeControllerManagerProbeName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to find")
	assert.Contains(t, err.Error(), "c-mine/node-a")
}

func TestImportedAdapter_ComponentTLSSettings_MalformedJSON(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "c-mine",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "node-a",
			},
		},
	}

	node := &mgmtv3.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-a",
			Namespace: "c-mine",
		},
		Status: mgmtv3.NodeStatus{
			NodeAnnotations: map[string]string{
				rke2NodeArgsAnnotation: `{this is not valid JSON`,
			},
		},
	}

	nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil)

	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}
	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
			Status:     mgmtv3.ClusterStatus{Provider: "rke2"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Mgmt:       stubMgmt,
				RESTMapper: &fakeRESTMapper{},
			},
		},
	}

	_, err := adapter.ComponentTLSSettings(secret, KubeSchedulerProbeName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse")
	assert.Contains(t, err.Error(), "rke2.io/node-args")
}

func TestImportedAdapter_ComponentTLSSettings_IgnoresMalformedNodeEnv(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "c-mine",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "node-a",
			},
		},
	}

	node := &mgmtv3.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-a",
			Namespace: "c-mine",
		},
		Status: mgmtv3.NodeStatus{
			NodeAnnotations: map[string]string{
				rke2NodeArgsAnnotation: `[]`,
				rke2NodeEnvAnnotation:  `{invalid-json`,
			},
		},
	}

	nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil).Times(2)

	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}
	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
			Status:     mgmtv3.ClusterStatus{Provider: "rke2"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Mgmt:       stubMgmt,
				RESTMapper: &fakeRESTMapper{},
			},
		},
	}

	settings, err := adapter.ComponentTLSSettings(secret, KubeControllerManagerProbeName)
	assert.NoError(t, err)
	assert.Equal(t, ComponentTLSSettings{}, settings)

	dataDir, err := adapter.DistroDataDirectory(secret)
	assert.Error(t, err)
	assert.Empty(t, dataDir)
}

func TestImportedAdapter_DistroDataDirectory_ReturnsMalformedConfigurationErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		args string
		env  string
	}{
		{"malformed args", `{invalid-json`, ``},
		{"malformed env", `[]`, `{invalid-json`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: "machine-plan", Namespace: "c-mine",
				Labels: map[string]string{
					planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
					planv1alpha1.MachineLifecycleKindLabel:  "Machine",
					planv1alpha1.MachineLifecycleNameLabel:  "node-a",
				},
			}}
			node := &mgmtv3.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-a", Namespace: "c-mine"},
				Status: mgmtv3.NodeStatus{NodeAnnotations: map[string]string{
					rke2NodeArgsAnnotation: tt.args,
					rke2NodeEnvAnnotation:  tt.env,
				}},
			}
			nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil)

			adapter := &ImportedAdapter{
				cluster: &mgmtv3.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
					Status:     mgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
				},
				clients: &wrangler.CAPIContext{
					Context: &wrangler.Context{
						Mgmt:       &stubMgmtInterface{nodeCache: nodeCache},
						RESTMapper: &fakeRESTMapper{},
					},
				},
			}

			dataDir, err := adapter.DistroDataDirectory(secret)
			assert.Error(t, err)
			assert.Empty(t, dataDir)
		})
	}
}

func TestImportedAdapter_DistroDataDirectory_NoLifecycleLabels(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "c-mine",
		},
	}

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
			Status:     mgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				RESTMapper: &fakeRESTMapper{},
			},
		},
	}

	// No lifecycle labels means managementNodeForSecret can't resolve a management Node, so
	// there is no way to know whether a custom data directory was configured. The default
	// directory must not be guessed in that case.
	dataDir, err := adapter.DistroDataDirectory(secret)
	assert.Error(t, err)
	assert.Empty(t, dataDir)
}

func TestImportedAdapter_DistroDataDirectory_UsesArgsWhenNodeEnvIsMalformed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "c-mine",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "node-a",
			},
		},
	}
	node := &mgmtv3.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-a", Namespace: "c-mine"},
		Status: mgmtv3.NodeStatus{
			NodeAnnotations: map[string]string{
				rke2NodeArgsAnnotation: `["--data-dir","/custom/from/args"]`,
				rke2NodeEnvAnnotation:  `{invalid-json`,
			},
		},
	}
	nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil)

	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
			Status:     mgmtv3.ClusterStatus{Provider: capr.RuntimeRKE2},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Mgmt:       &stubMgmtInterface{nodeCache: nodeCache},
				RESTMapper: &fakeRESTMapper{},
			},
		},
	}

	// A malformed environment annotation is irrelevant once CLI args already resolve the
	// effective data directory, since RKE2/K3s CLI arguments always outrank the environment.
	dataDir, err := adapter.DistroDataDirectory(secret)
	assert.NoError(t, err)
	assert.Equal(t, "/custom/from/args", dataDir)
}

func TestImportedAdapter_DistroDataDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime string
		args    []string
		env     map[string]string
		want    string
	}{
		{
			name:    "RKE2 args override env",
			runtime: capr.RuntimeRKE2,
			args:    []string{"--data-dir", "/custom/from/args"},
			env:     map[string]string{"RKE2_DATA_DIR": "/custom/from/env"},
			want:    "/custom/from/args",
		},
		{
			name:    "K3S args override env",
			runtime: capr.RuntimeK3S,
			args:    []string{"-d=/custom/from/args"},
			env:     map[string]string{"K3S_DATA_DIR": "/custom/from/env"},
			want:    "/custom/from/args",
		},
		{
			name:    "last data-dir argument wins across aliases",
			runtime: capr.RuntimeRKE2,
			args:    []string{"--data-dir", "/first", "-d", "/second", "--data-dir=/third"},
			want:    "/third",
		},
		{
			name:    "RKE2 defaults without configuration",
			runtime: capr.RuntimeRKE2,
			want:    defaultRKE2DataDirectory,
		},
		{
			name:    "RKE2 environment fallback",
			runtime: capr.RuntimeRKE2,
			env:     map[string]string{"RKE2_DATA_DIR": "/custom/from/env"},
			want:    "/custom/from/env",
		},
		{
			name:    "K3S environment fallback",
			runtime: capr.RuntimeK3S,
			env:     map[string]string{"K3S_DATA_DIR": "/custom/from/env"},
			want:    "/custom/from/env",
		},
		{
			name:    "K3S defaults without configuration",
			runtime: capr.RuntimeK3S,
			want:    defaultK3sDataDirectory,
		},
		{
			name:    "RKE2 environment does not affect K3S",
			runtime: capr.RuntimeK3S,
			env:     map[string]string{"RKE2_DATA_DIR": "/should/be/ignored"},
			want:    defaultK3sDataDirectory,
		},
		{
			name:    "K3S environment does not affect RKE2",
			runtime: capr.RuntimeRKE2,
			env:     map[string]string{"K3S_DATA_DIR": "/should/be/ignored"},
			want:    defaultRKE2DataDirectory,
		},
		{
			name:    "empty environment value is ignored",
			runtime: capr.RuntimeRKE2,
			env:     map[string]string{"RKE2_DATA_DIR": ""},
			want:    defaultRKE2DataDirectory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)
			annotations := map[string]string{}
			argsAnnotation, envAnnotation := rke2NodeArgsAnnotation, rke2NodeEnvAnnotation
			if tt.runtime == capr.RuntimeK3S {
				argsAnnotation, envAnnotation = k3sNodeArgsAnnotation, k3sNodeEnvAnnotation
			}
			if tt.args != nil {
				encoded, err := json.Marshal(tt.args)
				assert.NoError(t, err)
				annotations[argsAnnotation] = string(encoded)
			}
			if tt.env != nil {
				encoded, err := json.Marshal(tt.env)
				assert.NoError(t, err)
				annotations[envAnnotation] = string(encoded)
			}

			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: "machine-plan", Namespace: "c-mine",
				Labels: map[string]string{
					planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
					planv1alpha1.MachineLifecycleKindLabel:  "Machine",
					planv1alpha1.MachineLifecycleNameLabel:  "node-a",
				},
			}}
			nodeCache.EXPECT().Get("c-mine", "node-a").Return(&mgmtv3.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-a", Namespace: "c-mine"},
				Status:     mgmtv3.NodeStatus{NodeAnnotations: annotations},
			}, nil)

			adapter := &ImportedAdapter{
				cluster: &mgmtv3.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
					Status:     mgmtv3.ClusterStatus{Provider: tt.runtime},
				},
				clients: &wrangler.CAPIContext{
					Context: &wrangler.Context{
						Mgmt:       &stubMgmtInterface{nodeCache: nodeCache},
						RESTMapper: &fakeRESTMapper{},
					},
				},
			}

			dataDir, err := adapter.DistroDataDirectory(secret)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, dataDir)
		})
	}
}

// --- RenderProbes ------------------------------------------------------

func TestImportedAdapter_RenderProbes_UsesConfiguredComponentTLSSettings(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "c-mine",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "node-a",
				capr.ControlPlaneRoleLabel:              "true",
			},
		},
	}

	node := &mgmtv3.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-a",
			Namespace: "c-mine",
		},
		Spec: mgmtv3.NodeSpec{ControlPlane: true},
		Status: mgmtv3.NodeStatus{
			NodeAnnotations: map[string]string{
				// A custom secure-port and TLS cert for controller-manager; scheduler only
				// overrides its secure-port and keeps the default cert path. No --cni is set,
				// so this node's own RKE2 server config resolves to the Canal default.
				rke2NodeArgsAnnotation: `["--kube-controller-manager-arg","secure-port=10261",` +
					`"--kube-controller-manager-arg","tls-cert-file=/custom/kcm.crt",` +
					`"--kube-scheduler-arg","secure-port=10262"]`,
			},
		},
	}

	nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil).AnyTimes()
	nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{node}, nil)

	stubMgmt := &stubMgmtInterface{nodeCache: nodeCache}
	adapter := &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
			Status:     mgmtv3.ClusterStatus{Provider: "rke2"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Mgmt:       stubMgmt,
				RESTMapper: &fakeRESTMapper{},
			},
		},
	}

	probes, err := adapter.RenderProbes(secret, false)
	assert.NoError(t, err)
	assert.NotContains(t, probes, CalicoProbeName)

	kcm := probes[KubeControllerManagerProbeName]
	assert.Equal(t, "https://127.0.0.1:10261/healthz", kcm.HTTPGetAction.URL)
	assert.Equal(t, "/custom/kcm.crt", kcm.HTTPGetAction.CACert)

	scheduler := probes[KubeSchedulerProbeName]
	assert.Equal(t, "https://127.0.0.1:10262/healthz", scheduler.HTTPGetAction.URL)
	assert.Equal(t, "/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.crt", scheduler.HTTPGetAction.CACert)
}

// calicoTestSecret builds a machine-plan secret with lifecycle labels pointing at machineName,
// plus any extra role/OS labels needed by a Calico-probe test case.
func calicoTestSecret(machineName string, roleLabels map[string]string) *corev1.Secret {
	labels := map[string]string{
		planv1alpha1.MachineLifecycleGroupLabel: "management.cattle.io",
		planv1alpha1.MachineLifecycleKindLabel:  "Machine",
		planv1alpha1.MachineLifecycleNameLabel:  machineName,
	}
	maps.Copy(labels, roleLabels)
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "c-mine",
			Labels:    labels,
		},
	}
}

// calicoTestNode builds a management Node fixture with the given RKE2 role Spec and raw
// rke2.io/node-args / rke2.io/node-env annotation values ("" omits the annotation entirely).
func calicoTestNode(name string, spec mgmtv3.NodeSpec, args, env string) *mgmtv3.Node {
	annotations := map[string]string{}
	if args != "" {
		annotations[rke2NodeArgsAnnotation] = args
	}
	if env != "" {
		annotations[rke2NodeEnvAnnotation] = env
	}
	return &mgmtv3.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "c-mine"},
		Spec:       spec,
		Status:     mgmtv3.NodeStatus{NodeAnnotations: annotations},
	}
}

func calicoTestAdapter(provider string, nodeCache generic.CacheInterface[*mgmtv3.Node]) *ImportedAdapter {
	return &ImportedAdapter{
		cluster: &mgmtv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine"},
			Status:     mgmtv3.ClusterStatus{Provider: provider},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Mgmt:       &stubMgmtInterface{nodeCache: nodeCache},
				RESTMapper: &fakeRESTMapper{},
			},
		},
	}
}

func jsonArgs(t *testing.T, args []string) string {
	t.Helper()
	encoded, err := json.Marshal(args)
	assert.NoError(t, err)
	return string(encoded)
}

// TestImportedAdapter_RenderProbes_CalicoProbeSelection covers eligibility and single-node
// clusters, where the target of the plan is also the cluster's only RKE2 server — so the node's
// own --cni configuration is both the target's and the cluster's effective CNI.
func TestImportedAdapter_RenderProbes_CalicoProbeSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		provider       string
		labels         map[string]string
		spec           mgmtv3.NodeSpec
		nodeArgs       []string
		callsCNILookup bool
		wantCalico     bool
	}{
		{
			name:           "RKE2 --cni calico includes Calico",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.ControlPlaneRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{ControlPlane: true},
			nodeArgs:       []string{"server", "--cni", "calico"},
			callsCNILookup: true,
			wantCalico:     true,
		},
		{
			name:           "RKE2 --cni multus,calico includes Calico",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.ControlPlaneRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{ControlPlane: true},
			nodeArgs:       []string{"server", "--cni", "multus,calico"},
			callsCNILookup: true,
			wantCalico:     true,
		},
		{
			name:           "RKE2 --cni=calico includes Calico",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.ControlPlaneRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{ControlPlane: true},
			nodeArgs:       []string{"server", "--cni=calico"},
			callsCNILookup: true,
			wantCalico:     true,
		},
		{
			name:           "RKE2 --cni canal does not include Calico",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.ControlPlaneRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{ControlPlane: true},
			nodeArgs:       []string{"server", "--cni", "canal"},
			callsCNILookup: true,
			wantCalico:     false,
		},
		{
			name:           "RKE2 with no --cni does not include Calico",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.ControlPlaneRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{ControlPlane: true},
			nodeArgs:       []string{"server"},
			callsCNILookup: true,
			wantCalico:     false,
		},
		{
			name:           "RKE2 --cni cilium does not include Calico",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.ControlPlaneRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{ControlPlane: true},
			nodeArgs:       []string{"server", "--cni", "cilium"},
			callsCNILookup: true,
			wantCalico:     false,
		},
		{
			name:           "K3s skips RKE2 CNI lookup",
			provider:       capr.RuntimeK3S,
			labels:         map[string]string{capr.ControlPlaneRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{ControlPlane: true},
			nodeArgs:       []string{"server"},
			callsCNILookup: false,
			wantCalico:     false,
		},
		{
			name:           "etcd-only RKE2 node with Calico does not include Calico",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.EtcdRoleLabel: "true"},
			spec:           mgmtv3.NodeSpec{Etcd: true},
			nodeArgs:       []string{"server", "--cni", "calico"},
			callsCNILookup: false,
			wantCalico:     false,
		},
		{
			name:           "Windows worker skips RKE2 CNI lookup",
			provider:       capr.RuntimeRKE2,
			labels:         map[string]string{capr.WorkerRoleLabel: "true", capr.CattleOSLabel: "windows"},
			spec:           mgmtv3.NodeSpec{Worker: true},
			nodeArgs:       []string{"agent"},
			callsCNILookup: false,
			wantCalico:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

			secret := calicoTestSecret("node-a", tt.labels)

			argsAnnotation := rke2NodeArgsAnnotation
			if tt.provider == capr.RuntimeK3S {
				argsAnnotation = k3sNodeArgsAnnotation
			}
			node := &mgmtv3.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "node-a", Namespace: "c-mine"},
				Spec:       tt.spec,
				Status: mgmtv3.NodeStatus{
					NodeAnnotations: map[string]string{
						argsAnnotation: jsonArgs(t, tt.nodeArgs),
					},
				},
			}
			nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil).AnyTimes()
			if tt.callsCNILookup {
				nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{node}, nil)
			}

			probes, err := calicoTestAdapter(tt.provider, nodeCache).RenderProbes(secret, false)
			assert.NoError(t, err)
			if tt.wantCalico {
				assert.Contains(t, probes, CalicoProbeName)
			} else {
				assert.NotContains(t, probes, CalicoProbeName)
			}
		})
	}
}

// TestImportedAdapter_RenderProbes_CalicoAppliesToWorkerFromServerConfig proves that Calico
// applicability comes from the cluster's RKE2 server configuration, not from the target node's
// own arguments: a plain worker running `rke2 agent` never repeats --cni, but must still receive
// the Calico probe when a server node in the same cluster selected Calico.
func TestImportedAdapter_RenderProbes_CalicoAppliesToWorkerFromServerConfig(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

	secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})

	worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")
	server := calicoTestNode("node-server", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server", "--cni", "calico"}), "")

	nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
	nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{worker, server}, nil)

	probes, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
	assert.NoError(t, err)
	assert.Contains(t, probes, CalicoProbeName)
}

// TestImportedAdapter_RenderProbes_CalicoServerEnvironmentPrecedence covers the precedence
// between a server's explicit --cni argument and its RKE2_CNI environment fallback, and proves
// environment parsing is skipped (and therefore cannot block rendering) once --cni is present.
func TestImportedAdapter_RenderProbes_CalicoServerEnvironmentPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		env        string
		wantCalico bool
	}{
		{
			name:       "environment fallback used when --cni is absent",
			args:       []string{"server"},
			env:        `{"RKE2_CNI":"calico"}`,
			wantCalico: true,
		},
		{
			name:       "explicit --cni always outranks the environment",
			args:       []string{"server", "--cni", "canal"},
			env:        `{"RKE2_CNI":"calico"}`,
			wantCalico: false,
		},
		{
			name:       "malformed environment does not block an explicit --cni",
			args:       []string{"server", "--cni", "calico"},
			env:        `{invalid-json`,
			wantCalico: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

			// The target is a plain worker, kept separate from the server whose args/env are
			// under test — a malformed server rke2.io/node-env must not affect resolving the
			// target's own data directory.
			secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})
			worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")
			server := calicoTestNode("node-server", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, tt.args), tt.env)

			nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
			nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{worker, server}, nil)

			probes, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
			assert.NoError(t, err)
			if tt.wantCalico {
				assert.Contains(t, probes, CalicoProbeName)
			} else {
				assert.NotContains(t, probes, CalicoProbeName)
			}
		})
	}
}

// TestImportedAdapter_RenderProbes_CalicoCNILookupFailures covers the paths where Rancher cannot
// safely determine the cluster's effective RKE2 CNI, and must return an error instead of guessing
// — the operation retries rather than silently skipping a probe the cluster may actually need.
func TestImportedAdapter_RenderProbes_CalicoCNILookupFailures(t *testing.T) {
	t.Parallel()

	t.Run("no suitable server node", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})
		worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")

		nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{worker}, nil)

		_, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.Error(t, err)
	})

	t.Run("server has not reported runtime arguments yet", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		secret := calicoTestSecret("node-a", map[string]string{capr.ControlPlaneRoleLabel: "true"})
		node := calicoTestNode("node-a", mgmtv3.NodeSpec{ControlPlane: true}, "", "")

		nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil).AnyTimes()
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{node}, nil)

		_, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "has not reported its runtime arguments yet")
	})

	t.Run("management node list failure propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		secret := calicoTestSecret("node-a", map[string]string{capr.ControlPlaneRoleLabel: "true"})
		node := calicoTestNode("node-a", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server"}), "")
		listErr := errors.New("list failed")

		nodeCache.EXPECT().Get("c-mine", "node-a").Return(node, nil).AnyTimes()
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return(nil, listErr)

		_, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.Error(t, err)
		assert.ErrorIs(t, err, listErr)
	})

	t.Run("malformed required server node-args annotation propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		// The target is a plain worker with valid arguments; only the (separate) server's
		// node-args annotation is malformed.
		secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})
		worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")
		server := calicoTestNode("node-server", mgmtv3.NodeSpec{ControlPlane: true}, `{invalid-json`, "")

		nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{worker, server}, nil)

		_, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.Error(t, err)
	})

	t.Run("malformed server environment required for fallback propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		// The server has reported arguments but no explicit CNI, so resolving its
		// RKE2_CNI fallback is required. Malformed environment data is not safe to ignore.
		secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})
		worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")
		server := calicoTestNode("node-server", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server"}), `{invalid-json`)

		nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{worker, server}, nil)

		_, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "parsing runtime environment")
	})

	t.Run("deleting server is ignored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})
		worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")
		deletingServer := calicoTestNode("node-deleting", mgmtv3.NodeSpec{ControlPlane: true}, `{invalid-json`, "")
		deletionTime := metav1.Now()
		deletingServer.DeletionTimestamp = &deletionTime
		liveServer := calicoTestNode("node-live", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server", "--cni", "canal"}), "")

		nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{deletingServer, worker, liveServer}, nil)

		probes, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.NoError(t, err)
		assert.NotContains(t, probes, CalicoProbeName)
	})

	t.Run("equivalent server CNI selections do not conflict", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})
		worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")
		serverA := calicoTestNode("node-a", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server", "--cni", "multus", "--cni", "calico"}), "")
		serverB := calicoTestNode("node-b", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server", "--cni", "multus,calico"}), "")

		nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{serverB, worker, serverA}, nil)

		probes, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.NoError(t, err)
		assert.Contains(t, probes, CalicoProbeName)
	})

	t.Run("conflicting server CNI selections", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		secret := calicoTestSecret("node-a", map[string]string{capr.ControlPlaneRoleLabel: "true"})
		nodeA := calicoTestNode("node-a", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server", "--cni", "calico"}), "")
		nodeB := calicoTestNode("node-b", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server", "--cni", "canal"}), "")

		nodeCache.EXPECT().Get("c-mine", "node-a").Return(nodeA, nil).AnyTimes()
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{nodeA, nodeB}, nil)

		_, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "conflicting effective CNI selections")
	})

	t.Run("etcd-only server is ignored in split-role cluster", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		nodeCache := ctrlfake.NewMockCacheInterface[*mgmtv3.Node](ctrl)

		// Only control-plane nodes deploy the bundled CNI. An etcd-only node reporting plain
		// server args with no CNI setting must not be read as a Canal selection, nor compared
		// against the control-plane node's Calico selection as a conflict.
		secret := calicoTestSecret("node-worker", map[string]string{capr.WorkerRoleLabel: "true"})
		worker := calicoTestNode("node-worker", mgmtv3.NodeSpec{Worker: true}, jsonArgs(t, []string{"agent"}), "")
		etcdOnly := calicoTestNode("node-etcd", mgmtv3.NodeSpec{Etcd: true}, jsonArgs(t, []string{"server"}), "")
		controlPlane := calicoTestNode("node-cp", mgmtv3.NodeSpec{ControlPlane: true}, jsonArgs(t, []string{"server", "--cni", "calico"}), "")

		nodeCache.EXPECT().Get("c-mine", "node-worker").Return(worker, nil)
		nodeCache.EXPECT().List("c-mine", gomock.Any()).Return([]*mgmtv3.Node{etcdOnly, worker, controlPlane}, nil)

		probes, err := calicoTestAdapter(capr.RuntimeRKE2, nodeCache).RenderProbes(secret, false)
		assert.NoError(t, err)
		assert.Contains(t, probes, CalicoProbeName)
	})
}

// --- arguments tests --------------------------------

func TestArgumentsLast(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		names []string
		value string
	}{
		{
			name:  "split long option",
			args:  []string{"server", "--data-dir", "/custom/rke2"},
			names: []string{"--data-dir", "-d"},
			value: "/custom/rke2",
		},
		{
			name:  "combined short option",
			args:  []string{"-d=/custom/rke2"},
			names: []string{"--data-dir", "-d"},
			value: "/custom/rke2",
		},
		{
			name:  "last option wins across aliases",
			args:  []string{"-d", "/first", "--data-dir=/second"},
			names: []string{"--data-dir", "-d"},
			value: "/second",
		},
		{
			name:  "last option wins with mixed aliases",
			args:  []string{"--data-dir", "/first", "-d", "/second", "--data-dir=/third"},
			names: []string{"--data-dir", "-d"},
			value: "/third",
		},
		{
			name:  "missing option value",
			args:  []string{"--data-dir"},
			names: []string{"--data-dir", "-d"},
		},
		{
			name:  "does not match option prefix",
			args:  []string{"--data-directory=/custom/rke2"},
			names: []string{"--data-dir", "-d"},
		},
		{
			name:  "empty value is ignored",
			args:  []string{"--data-dir", "/first", "-d", ""},
			names: []string{"--data-dir", "-d"},
			value: "/first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.value, arguments(tt.args).Last(tt.names...))
		})
	}
}

func TestArgumentsValues(t *testing.T) {
	t.Parallel()

	args := arguments([]string{
		"--kube-scheduler-arg", "secure-port=10262",
		"--kube-scheduler-arg=tls-cert-file=/custom/scheduler.crt",
		"--kube-controller-manager-arg", "secure-port=10261",
		"--kube-scheduler-arg", "tls-private-key-file=/custom/scheduler.key",
		"--kube-scheduler-args=not-a-match",
		"--kube-scheduler-arg",
	})

	assert.Equal(t, []string{
		"secure-port=10262",
		"tls-cert-file=/custom/scheduler.crt",
		"tls-private-key-file=/custom/scheduler.key",
	}, args.Values("--kube-scheduler-arg"))
	assert.Nil(t, args.Values("--missing"))
}
