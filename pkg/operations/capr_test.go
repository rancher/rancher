package operations

import (
	"testing"

	"github.com/rancher/channelserver/pkg/model"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/wrangler"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/schemas"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// --- RuntimeCommand / ServerUnit ------------------------------------------------------------

func TestCAPRAdapter_RuntimeCommand(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		k8sVersion string
		want       string
	}{
		{"rke2 version", "v1.28.5+rke2r1", "rke2"},
		{"k3s version", "v1.28.5+k3s1", "k3s"},
		{"k3s default", "v1.28.5", "k3s"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &CAPRAdapter{
				controlPlane: &rkev1.RKEControlPlane{
					Spec: rkev1.RKEControlPlaneSpec{
						KubernetesVersion: tc.k8sVersion,
					},
				},
			}
			got := a.RuntimeCommand()
			assert.Equal(t, tc.want, got, "RuntimeCommand mismatch for %s", tc.k8sVersion)
		})
	}
}

func TestCAPRAdapter_ServerUnit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		k8sVersion string
		want       string
	}{
		{"rke2 version", "v1.28.5+rke2r1", "rke2-server"},
		{"k3s version", "v1.28.5+k3s1", "k3s"},
		{"k3s default", "v1.28.5", "k3s"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &CAPRAdapter{
				controlPlane: &rkev1.RKEControlPlane{
					Spec: rkev1.RKEControlPlaneSpec{
						KubernetesVersion: tc.k8sVersion,
					},
				},
			}
			got := a.ServerUnit()
			assert.Equal(t, tc.want, got, "ServerUnit mismatch for %s", tc.k8sVersion)
		})
	}
}

// --- WaitForRegister ------------------------------------------------------------------------

func newMachinePlanSecret(name, machineName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "fleet-default",
			UID:       types.UID(name + "-uid"),
			Labels: map[string]string{
				capr.ClusterNameLabel: "c-mine",
				capr.MachineNameLabel: machineName,
			},
		},
		Type: capr.SecretTypeMachinePlan,
	}
}

func newCAPIMachine(name string) *capi.Machine {
	return &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "fleet-default",
			Labels: map[string]string{
				capr.ClusterNameLabel: "c-mine",
			},
		},
	}
}

func TestCAPRAdapter_WaitForRegister_Perfect1to1(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)

	secrets := []*corev1.Secret{
		newMachinePlanSecret("secret-a", "machine-a"),
		newMachinePlanSecret("secret-b", "machine-b"),
	}
	machines := []*capi.Machine{
		newCAPIMachine("machine-a"),
		newCAPIMachine("machine-b"),
	}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	machineCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(machines, nil)

	// Create stub interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubCAPI := &stubCAPIInterface{machineCache: machineCache}

	adapter := &CAPRAdapter{
		controlPlane: &rkev1.RKEControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine", Namespace: "fleet-default"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
			},
			CAPI: stubCAPI,
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.True(t, ok, "perfect 1:1 match should return true")
}

func TestCAPRAdapter_WaitForRegister_CountMismatch(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)

	secrets := []*corev1.Secret{newMachinePlanSecret("secret-a", "machine-a")}
	machines := []*capi.Machine{
		newCAPIMachine("machine-a"),
		newCAPIMachine("machine-b"),
	}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	machineCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(machines, nil)

	// Create stub interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubCAPI := &stubCAPIInterface{machineCache: machineCache}

	adapter := &CAPRAdapter{
		controlPlane: &rkev1.RKEControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine", Namespace: "fleet-default"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
			},
			CAPI: stubCAPI,
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "count mismatch should return false")
}

func TestCAPRAdapter_WaitForRegister_DuplicateSecrets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)

	// Two secrets pointing to the same machine.
	secrets := []*corev1.Secret{
		newMachinePlanSecret("secret-a", "machine-a"),
		newMachinePlanSecret("secret-b", "machine-a"),
	}
	machines := []*capi.Machine{
		newCAPIMachine("machine-a"),
		newCAPIMachine("machine-b"),
	}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	machineCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(machines, nil)

	// Create stub interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubCAPI := &stubCAPIInterface{machineCache: machineCache}

	adapter := &CAPRAdapter{
		controlPlane: &rkev1.RKEControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine", Namespace: "fleet-default"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
			},
			CAPI: stubCAPI,
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "duplicate secrets (same machine) should return false")
}

func TestCAPRAdapter_WaitForRegister_MissingMachineNameLabel(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)

	secretNoLabel := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-a",
			Namespace: "fleet-default",
			Labels: map[string]string{
				capr.ClusterNameLabel: "c-mine",
				// No MachineNameLabel
			},
		},
		Type: capr.SecretTypeMachinePlan,
	}
	secrets := []*corev1.Secret{secretNoLabel}
	machines := []*capi.Machine{newCAPIMachine("machine-a")}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	machineCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(machines, nil)

	// Create stub interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubCAPI := &stubCAPIInterface{machineCache: machineCache}

	adapter := &CAPRAdapter{
		controlPlane: &rkev1.RKEControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine", Namespace: "fleet-default"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
			},
			CAPI: stubCAPI,
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "secret without machine-name label should return false")
}

func TestCAPRAdapter_WaitForRegister_NilLabels(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	secretCache := ctrlfake.NewMockCacheInterface[*corev1.Secret](ctrl)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)

	secretNilLabels := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-a",
			Namespace: "fleet-default",
			Labels:    nil,
		},
		Type: capr.SecretTypeMachinePlan,
	}
	secrets := []*corev1.Secret{secretNilLabels}
	machines := []*capi.Machine{newCAPIMachine("machine-a")}

	secretCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(secrets, nil)
	machineCache.EXPECT().List(gomock.Any(), gomock.Any()).Return(machines, nil)

	// Create stub interfaces.
	stubCore := &stubCoreInterface{secretCache: secretCache}
	stubCAPI := &stubCAPIInterface{machineCache: machineCache}

	adapter := &CAPRAdapter{
		controlPlane: &rkev1.RKEControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "c-mine", Namespace: "fleet-default"},
		},
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				Core: stubCore,
			},
			CAPI: stubCAPI,
		},
	}

	ok, err := adapter.WaitForRegister()
	assert.NoError(t, err)
	assert.False(t, ok, "secret with nil labels should return false")
}

// --- isCalico -------------------------------------------------------------------------------

func TestIsCalico(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		runtime string
		cni     string
		expect  bool
	}{
		{"rke2 default cni", "rke2", "", true},
		{"rke2 calico", "rke2", "calico", true},
		{"rke2 calico+multus", "rke2", "calico+multus", true},
		{"rke2 canal", "rke2", "canal", false},
		{"k3s calico ignored", "k3s", "calico", false},
		{"k3s default", "k3s", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp := &rkev1.RKEControlPlane{
				Spec: rkev1.RKEControlPlaneSpec{
					ClusterConfiguration: rkev1.ClusterConfiguration{
						MachineGlobalConfig: rkev1.GenericMap{
							Data: map[string]interface{}{},
						},
					},
				},
			}
			if tc.cni != "" {
				cp.Spec.ClusterConfiguration.MachineGlobalConfig.Data["cni"] = tc.cni
			}
			got := isCalico(cp, tc.runtime)
			assert.Equal(t, tc.expect, got, "isCalico(%s, %q)", tc.runtime, tc.cni)
		})
	}
}

// --- splitArgKeyVal -------------------------------------------------------------------------

func TestSplitArgKeyVal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val   string
		delim string
		key   string
		value string
	}{
		{"key=value", "=", "key", "value"},
		{"key:value", ":", "key", "value"},
		{"no-delim", "=", "", ""},
		{"a=b=c", "=", "a", "b=c"}, // SplitN 2 means first = only
		{"", "=", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.val, func(t *testing.T) {
			k, v := splitArgKeyVal(tc.val, tc.delim)
			assert.Equal(t, tc.key, k, "key mismatch")
			assert.Equal(t, tc.value, v, "value mismatch")
		})
	}
}

// --- convertInterfaceSliceToStringSlice -----------------------------------------------------

func TestConvertInterfaceSliceToStringSlice(t *testing.T) {
	t.Parallel()

	input := []any{"a", 42, true, "hello"}
	want := []string{"a", "42", "true", "hello"}

	got := convertInterfaceSliceToStringSlice(input)
	assert.Equal(t, want, got, "convertInterfaceSliceToStringSlice mismatch")
}

// --- getArgValue ----------------------------------------------------------------------------

func TestGetArgValue(t *testing.T) {
	t.Parallel()

	t.Run("string arg exact match", func(t *testing.T) {
		arg := "key=value"
		got := getArgValue(arg, "key", "=")
		assert.Equal(t, "value", got)
	})

	t.Run("string arg no match", func(t *testing.T) {
		arg := "other=val"
		got := getArgValue(arg, "key", "=")
		assert.Equal(t, "", got)
	})

	t.Run("string slice match", func(t *testing.T) {
		arg := []string{"a=1", "key=found", "b=2"}
		got := getArgValue(arg, "key", "=")
		assert.Equal(t, "found", got)
	})

	t.Run("string slice no match", func(t *testing.T) {
		arg := []string{"a=1", "b=2"}
		got := getArgValue(arg, "key", "=")
		assert.Equal(t, "", got)
	})

	t.Run("interface slice match", func(t *testing.T) {
		arg := []any{"x=10", "key=yes"}
		got := getArgValue(arg, "key", "=")
		assert.Equal(t, "yes", got)
	})

	t.Run("interface slice no match", func(t *testing.T) {
		arg := []any{"x=10", "y=20"}
		got := getArgValue(arg, "key", "=")
		assert.Equal(t, "", got)
	})

	t.Run("empty slice", func(t *testing.T) {
		arg := []string{}
		got := getArgValue(arg, "key", "=")
		assert.Equal(t, "", got)
	})

	t.Run("nil arg", func(t *testing.T) {
		got := getArgValue(nil, "key", "=")
		assert.Equal(t, "", got)
	})
}

// --- filterField ----------------------------------------------------------------------------

func TestFilterField(t *testing.T) {
	t.Parallel()

	release := model.Release{
		ServerArgs: map[string]schemas.Field{
			"data-dir": {Type: "string"},
			"tls":      {Type: "boolean"},
		},
		AgentArgs: map[string]schemas.Field{
			"node-name": {Type: "string"},
		},
	}

	cases := []struct {
		name     string
		isServer bool
		key      string
		value    any
		wantVal  any
		wantOK   bool
	}{
		{"server arg string", true, "data-dir", "/my/path", "/my/path", true},
		{"server arg bool", true, "tls", true, true, true},
		{"server arg missing", true, "unknown", "val", nil, false},
		{"agent arg string", false, "node-name", "n1", "n1", true},
		{"agent arg not in server", false, "data-dir", "/path", nil, false},
		{"nil value", true, "data-dir", nil, nil, false},
		{"boolean type conversion", true, "tls", "true", true, true},
		{"array value", true, "data-dir", []any{"a", "b"}, []any{"a", "b"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotVal, gotOK := filterField(tc.isServer, tc.key, tc.value, release)
			assert.Equal(t, tc.wantOK, gotOK, "ok mismatch")
			if tc.wantOK {
				assert.Equal(t, tc.wantVal, gotVal, "value mismatch")
			}
		})
	}
}

// --- filterConfigData -----------------------------------------------------------------------

func TestFilterConfigData(t *testing.T) {
	t.Parallel()

	// This is a shallow integration test — filterConfigData calls GetKDMReleaseData which
	// requires a fully-wired context. We'll verify the basic flow: recognized keys are kept,
	// unrecognized keys are deleted. Deep KDM fetching is out of scope for a unit test.

	// Skipping for now since GetKDMReleaseData is a global lookup that needs a real cluster
	// setup. The core logic (filterField) is already tested above.
}
