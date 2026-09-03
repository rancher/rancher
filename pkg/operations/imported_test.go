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

	dataDir := adapter.DistroDataDirectory(secret)
	assert.Equal(t, defaultRKE2DataDirectory, dataDir)
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
		Status: mgmtv3.NodeStatus{
			NodeAnnotations: map[string]string{
				// A custom secure-port and TLS cert for controller-manager; scheduler only
				// overrides its secure-port and keeps the default cert path.
				rke2NodeArgsAnnotation: `["--kube-controller-manager-arg","secure-port=10261",` +
					`"--kube-controller-manager-arg","tls-cert-file=/custom/kcm.crt",` +
					`"--kube-scheduler-arg","secure-port=10262"]`,
			},
		},
	}

	// RenderProbes reads node args twice: once via DistroDataDirectory, once to compute the
	// effective component TLS settings for the probes.
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

	probes, err := adapter.RenderProbes(secret, false)
	assert.NoError(t, err)

	kcm := probes[KubeControllerManagerProbeName]
	assert.Equal(t, "https://127.0.0.1:10261/healthz", kcm.HTTPGetAction.URL)
	assert.Equal(t, "/custom/kcm.crt", kcm.HTTPGetAction.CACert)

	scheduler := probes[KubeSchedulerProbeName]
	assert.Equal(t, "https://127.0.0.1:10262/healthz", scheduler.HTTPGetAction.URL)
	assert.Equal(t, "/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.crt", scheduler.HTTPGetAction.CACert)
}

// --- importedDistroDataDirectory tests --------------------------------

func TestImportedDistroDataDirectory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		runtime string
		args    []string
		env     map[string]string
		want    string
	}{
		{
			name:    "RKE2_DATA_DIR env overrides args",
			runtime: capr.RuntimeRKE2,
			args:    []string{"--data-dir", "/custom/from/args"},
			env:     map[string]string{"RKE2_DATA_DIR": "/custom/from/env"},
			want:    "/custom/from/env",
		},
		{
			name:    "K3S_DATA_DIR env overrides args",
			runtime: capr.RuntimeK3S,
			args:    []string{"-d=/custom/from/args"},
			env:     map[string]string{"K3S_DATA_DIR": "/custom/from/env"},
			want:    "/custom/from/env",
		},
		{
			name:    "last data-dir argument wins across aliases",
			runtime: capr.RuntimeRKE2,
			args:    []string{"--data-dir", "/first", "-d", "/second", "--data-dir=/third"},
			want:    "/third",
		},
		{
			name:    "RKE2 defaults correctly when no config provided",
			runtime: capr.RuntimeRKE2,
			want:    "/var/lib/rancher/rke2",
		},
		{
			name:    "K3s defaults correctly when no config provided",
			runtime: capr.RuntimeK3S,
			want:    "/var/lib/rancher/k3s",
		},
		{
			name:    "RKE2 env var does not affect K3s",
			runtime: capr.RuntimeK3S,
			env:     map[string]string{"RKE2_DATA_DIR": "/should/be/ignored"},
			want:    "/var/lib/rancher/k3s",
		},
		{
			name:    "K3S env var does not affect RKE2",
			runtime: capr.RuntimeRKE2,
			env:     map[string]string{"K3S_DATA_DIR": "/should/be/ignored"},
			want:    "/var/lib/rancher/rke2",
		},
		{
			name:    "empty env var is ignored",
			runtime: capr.RuntimeRKE2,
			args:    []string{"--data-dir", "/custom"},
			env:     map[string]string{"RKE2_DATA_DIR": ""},
			want:    "/custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importedDistroDataDirectory(tt.runtime, tt.args, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
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
			assert.Equal(t, tt.value, newArguments(tt.args).Last(tt.names...))
		})
	}
}

func TestArgumentsValues(t *testing.T) {
	t.Parallel()

	args := newArguments([]string{
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
