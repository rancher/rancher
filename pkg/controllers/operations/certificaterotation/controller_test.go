package certificaterotation

import (
	"errors"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// stubAdapter is a minimal ops.Adapter implementation for testing component
// certificate cleanup plan construction.
type stubAdapter struct {
	runtime           string
	dataDir           string
	provisioningDir   string
	controllerManager ops.ComponentTLSSettings
	scheduler         ops.ComponentTLSSettings
	settingsErr       error
}

func (a *stubAdapter) RuntimeCommand() string                      { return a.runtime }
func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) string { return a.dataDir }
func (a *stubAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	return a.provisioningDir
}
func (a *stubAdapter) CertificateRotationComponentTLSSettings(_ *corev1.Secret, component string) (ops.ComponentTLSSettings, error) {
	if a.settingsErr != nil {
		return ops.ComponentTLSSettings{}, a.settingsErr
	}
	switch component {
	case ops.KubeControllerManagerProbeName:
		return a.controllerManager, nil
	case ops.KubeSchedulerProbeName:
		return a.scheduler, nil
	default:
		return ops.ComponentTLSSettings{}, nil
	}
}

// The remaining methods complete the ops.Adapter contract. Component cleanup
// does not call them, so the stub returns static, runtime-appropriate values.
func (a *stubAdapter) BeaconRef() (string, string)                        { return "", "" }
func (a *stubAdapter) EtcdSnapshotNamespace() string                      { return "" }
func (a *stubAdapter) ClusterObject() (*unstructured.Unstructured, error) { return nil, nil }
func (a *stubAdapter) WaitForRegister() (bool, error)                     { return true, nil }
func (a *stubAdapter) PauseCluster(bool) error                            { return nil }
func (a *stubAdapter) ServerUnit() string                                 { return a.runtime }
func (a *stubAdapter) RuntimeService(secret *corev1.Secret) string {
	if ops.IsControlPlane(secret) || ops.IsEtcd(secret) {
		return a.ServerUnit()
	}
	return a.runtime + "-agent"
}
func (a *stubAdapter) DistroServices(secret *corev1.Secret) []string {
	return ops.DistroServices(a.runtime, secret)
}
func (a *stubAdapter) ConfigFile(_ *corev1.Secret) string      { return "" }
func (a *stubAdapter) ConfigDirectory(_ *corev1.Secret) string { return "" }
func (a *stubAdapter) RenderProbes(*corev1.Secret, bool) (map[string]plan.Probe, error) {
	return map[string]plan.Probe{}, nil
}
func (a *stubAdapter) KubectlPath(_ *corev1.Secret) string    { return "" }
func (a *stubAdapter) KubeconfigPath(_ *corev1.Secret) string { return "" }
func (a *stubAdapter) FindOrElectLeader(string, ops.Filter) (*corev1.Secret, error) {
	return nil, nil
}
func (a *stubAdapter) GetServerURL(_ *corev1.Secret) string      { return "" }
func (a *stubAdapter) GetSupervisorPort(_ *corev1.Secret) string { return "" }
func (a *stubAdapter) LoopbackAddress(_ *corev1.Secret) string   { return "127.0.0.1" }
func (a *stubAdapter) ToS3ArgsEnvAndFiles(_ *corev1.Secret) ([]string, []string, []plan.File) {
	return nil, nil, nil
}

func TestComponentCertificateCleanupInstructions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		runtime          string
		services         []string
		controllerConfig ops.ComponentTLSSettings
		schedulerConfig  ops.ComponentTLSSettings
		expected         []string
	}{
		{
			name:    "RKE2 all services",
			runtime: capr.RuntimeRKE2,
			expected: []string{
				"/var/lib/rancher/rke2/server/tls/kube-controller-manager/kube-controller-manager.crt",
				"/var/lib/rancher/rke2/server/tls/kube-controller-manager/kube-controller-manager.key",
				"/var/lib/rancher/rke2/agent/pod-manifests/kube-controller-manager.yaml",
				"/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.crt",
				"/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.key",
				"/var/lib/rancher/rke2/agent/pod-manifests/kube-scheduler.yaml",
			},
		},
		{
			name:     "K3s scheduler only",
			runtime:  capr.RuntimeK3S,
			services: []string{"scheduler"},
			expected: []string{
				"/var/lib/rancher/k3s/server/tls/kube-scheduler/kube-scheduler.crt",
				"/var/lib/rancher/k3s/server/tls/kube-scheduler/kube-scheduler.key",
			},
		},
		{
			name:     "non component service",
			runtime:  capr.RuntimeRKE2,
			services: []string{"etcd"},
		},
		{
			name:     "complete custom controller-manager TLS suppresses its cleanup",
			runtime:  capr.RuntimeRKE2,
			services: []string{"controller-manager"},
			controllerConfig: ops.ComponentTLSSettings{
				TLSCertFile:       "/custom/kcm.crt",
				TLSPrivateKeyFile: "/custom/kcm.key",
			},
		},
		{
			name:     "custom controller-manager TLS does not suppress scheduler cleanup",
			runtime:  capr.RuntimeRKE2,
			services: []string{"controller-manager", "scheduler"},
			controllerConfig: ops.ComponentTLSSettings{
				TLSCertFile:       "/custom/kcm.crt",
				TLSPrivateKeyFile: "/custom/kcm.key",
			},
			expected: []string{
				"/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.crt",
				"/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.key",
				"/var/lib/rancher/rke2/agent/pod-manifests/kube-scheduler.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := &stubAdapter{
				runtime:           tt.runtime,
				dataDir:           "/var/lib/rancher/" + tt.runtime,
				provisioningDir:   "/var/lib/rancher/capr",
				controllerManager: tt.controllerConfig,
				scheduler:         tt.schedulerConfig,
			}
			s := &scope{
				op: &opv1alpha1.CertificateRotation{
					ObjectMeta: metav1.ObjectMeta{
						UID: "operation",
					},
				},
				adapter: adapter,
			}

			instructions, err := componentCertificateCleanupInstructions(s, &corev1.Secret{}, tt.services)
			assert.NoError(t, err)
			assert.Len(t, instructions, len(tt.expected))
			for i, instruction := range instructions {
				assert.GreaterOrEqual(t, len(instruction.Args), 4)
				assert.Equal(t, "rm", instruction.Args[len(instruction.Args)-4])
				assert.Equal(t, []string{"-f", tt.expected[i]}, instruction.Args[len(instruction.Args)-2:])
			}
		})
	}
}

func TestComponentCertificateCleanupInstructions_AdapterError(t *testing.T) {
	t.Parallel()

	s := &scope{
		op: &opv1alpha1.CertificateRotation{
			ObjectMeta: metav1.ObjectMeta{UID: "operation"},
		},
		adapter: &stubAdapter{
			runtime:         capr.RuntimeRKE2,
			dataDir:         "/var/lib/rancher/rke2",
			provisioningDir: "/var/lib/rancher/capr",
			settingsErr:     errors.New("settings failed"),
		},
	}

	instructions, err := componentCertificateCleanupInstructions(s, &corev1.Secret{}, nil)
	assert.Nil(t, instructions)
	assert.EqualError(t, err, "settings failed")
}

func TestCertificateRotationComponentProbes(t *testing.T) {
	t.Parallel()

	s := &scope{
		adapter: &stubAdapter{
			dataDir: "/var/lib/rancher/rke2",
			controllerManager: ops.ComponentTLSSettings{
				SecurePort:  "10261",
				TLSCertFile: "/custom/kube-controller-manager.crt",
			},
		},
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Namespace: "fleet-default",
		Name:      "machine-plan",
		Labels:    map[string]string{capr.ControlPlaneRoleLabel: "true"},
	}}
	probes := map[string]plan.Probe{
		ops.KubeControllerManagerProbeName: {
			HTTPGetAction: plan.HTTPGetAction{URL: "https://[::1]:10257/healthz"},
		},
		ops.KubeSchedulerProbeName: {
			HTTPGetAction: plan.HTTPGetAction{URL: "https://[::1]:10259/healthz"},
		},
	}

	got, err := renderCertificateRotationComponentProbes(s, secret, probes)
	assert.NoError(t, err)
	assert.Equal(t, "https://[::1]:10261/healthz", got[ops.KubeControllerManagerProbeName].HTTPGetAction.URL)
	assert.Equal(t, "/custom/kube-controller-manager.crt", got[ops.KubeControllerManagerProbeName].HTTPGetAction.CACert)
	assert.Equal(t, "https://[::1]:10259/healthz", got[ops.KubeSchedulerProbeName].HTTPGetAction.URL)
	assert.Equal(t, "/var/lib/rancher/rke2/server/tls/kube-scheduler/kube-scheduler.crt", got[ops.KubeSchedulerProbeName].HTTPGetAction.CACert)
}

func TestWindowsIdempotentRestartInstructions_UsesPassedRuntime(t *testing.T) {
	t.Parallel()

	instructions := windowsIdempotentRestartInstructions("certificate-rotation/restart", "operation", capr.RuntimeK3S)
	assert.Len(t, instructions, 1)
	assert.Contains(t, instructions[0].Args, capr.RuntimeK3S)
}

func TestCertificateRotationRuntimeInstructions_CustomDataDirWithServices(t *testing.T) {
	t.Parallel()

	instructions := certificateRotationRuntimeInstructions(
		"/var/lib/rancher/capr", "operation", capr.RuntimeRKE2, "/custom/data-dir",
		[]string{"etcd", "api-server"}, nil)
	assert.Len(t, instructions, 1)

	args := instructions[0].Args
	assert.Equal(t, []string{
		"certificate",
		"rotate",
		"--data-dir",
		"/custom/data-dir",
		"-s",
		"etcd",
		"-s",
		"api-server",
	}, args[len(args)-8:])
}

func TestCertificateRotationRuntimeInstructions_CustomDataDirNoServices(t *testing.T) {
	t.Parallel()

	instructions := certificateRotationRuntimeInstructions(
		"/var/lib/rancher/capr", "operation", capr.RuntimeRKE2, "/custom/data-dir",
		nil, nil)
	assert.Len(t, instructions, 1)

	args := instructions[0].Args
	assert.Equal(t, []string{
		"certificate",
		"rotate",
		"--data-dir",
		"/custom/data-dir",
	}, args[len(args)-4:])
	assert.NotContains(t, args, "-s")
}

func TestRKE2ManifestRemovalInstructions_DataDirWithSpacesIsNotInterpolated(t *testing.T) {
	t.Parallel()

	dataDir := "/var/lib/rancher/testing/certificate rotation"
	instructions := rke2ManifestRemovalInstructions("/var/lib/rancher/capr", "operation", dataDir)
	assert.Len(t, instructions, 1)

	instr := instructions[0]
	assert.Equal(t, "/bin/sh", instr.Command)

	args := instr.Args
	assert.Equal(t, []string{
		"-c",
		`rm -f -- "$1"/rke2-*.yaml`,
		"--",
		"/var/lib/rancher/testing/certificate rotation/server/manifests",
	}, args[len(args)-4:])

	for _, arg := range args[:len(args)-1] {
		assert.NotContains(t, arg, dataDir, "shell script/command arguments must not embed the data directory")
	}
}

func TestServicesApply(t *testing.T) {
	t.Parallel()

	controlPlane := certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"})
	etcd := certificateRotationSecret(map[string]string{capr.EtcdRoleLabel: "true"})
	worker := certificateRotationSecret(map[string]string{capr.WorkerRoleLabel: "true"})
	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}

	tests := []struct {
		name     string
		services []string
		secret   *corev1.Secret
		want     bool
	}{
		{name: "all services includes every node", secret: worker, want: true},
		{name: "scheduler selects control plane", services: []string{"scheduler"}, secret: controlPlane, want: true},
		{name: "scheduler excludes worker", services: []string{"scheduler"}, secret: worker},
		{name: "etcd selects etcd", services: []string{"etcd"}, secret: etcd, want: true},
		{name: "etcd excludes worker", services: []string{"etcd"}, secret: worker},
		{name: "supervisor selects control plane", services: []string{"supervisor"}, secret: controlPlane, want: true},
		{name: "supervisor selects etcd", services: []string{"supervisor"}, secret: etcd, want: true},
		{name: "supervisor excludes worker", services: []string{"supervisor"}, secret: worker},
		{name: "kubelet selects worker", services: []string{"kubelet"}, secret: worker, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, servicesApply(adapter, tt.services, tt.secret))
		})
	}
}

func TestServicesForNode_NarrowsRequestPerNodeRole(t *testing.T) {
	t.Parallel()

	etcd := certificateRotationSecret(map[string]string{capr.EtcdRoleLabel: "true"})
	controlPlane := certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"})
	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}

	requested := []string{"etcd", "scheduler"}

	// A request spanning both roles must be narrowed separately for each node: the etcd
	// node only receives "etcd", the control-plane node only receives "scheduler".
	assert.Equal(t, []string{"etcd"}, servicesForNode(adapter, requested, etcd))
	assert.Equal(t, []string{"scheduler"}, servicesForNode(adapter, requested, controlPlane))
}

func TestServicesForNode_EmptyRequestStaysEmpty(t *testing.T) {
	t.Parallel()

	controlPlane := certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"})
	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}

	// An empty request already means "rotate everything the runtime supports" to the
	// runtime command, so it must not be expanded into the node's full DistroServices list.
	assert.Nil(t, servicesForNode(adapter, nil, controlPlane))
}

// --- DistroServices ---------------------------------------------------------------------------

func TestDistroServices_RuntimeSpecificNamesDoNotCross(t *testing.T) {
	t.Parallel()

	controlPlane := certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"})

	rke2 := ops.DistroServices(capr.RuntimeRKE2, controlPlane)
	assert.Contains(t, rke2, "rke2-server")
	assert.Contains(t, rke2, "rke2-controller")
	assert.NotContains(t, rke2, "k3s-server", "RKE2 nodes must not expose K3s-specific service names")
	assert.NotContains(t, rke2, "k3s-controller", "RKE2 nodes must not expose K3s-specific service names")

	k3s := ops.DistroServices(capr.RuntimeK3S, controlPlane)
	assert.Contains(t, k3s, "k3s-server")
	assert.Contains(t, k3s, "k3s-controller")
	assert.NotContains(t, k3s, "rke2-server", "K3s nodes must not expose RKE2-specific service names")
	assert.NotContains(t, k3s, "rke2-controller", "K3s nodes must not expose RKE2-specific service names")
}

func TestDistroServices_RoleSpecificAvailability(t *testing.T) {
	t.Parallel()

	controlPlane := certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"})
	etcd := certificateRotationSecret(map[string]string{capr.EtcdRoleLabel: "true"})
	worker := certificateRotationSecret(map[string]string{capr.WorkerRoleLabel: "true"})

	// Worker-only nodes never own control-plane or etcd services.
	workerServices := ops.DistroServices(capr.RuntimeRKE2, worker)
	assert.Contains(t, workerServices, "rke2-server")
	assert.NotContains(t, workerServices, "scheduler")
	assert.NotContains(t, workerServices, "etcd")

	// Control-plane nodes own the API server components but not etcd.
	controlPlaneServices := ops.DistroServices(capr.RuntimeRKE2, controlPlane)
	assert.Contains(t, controlPlaneServices, "scheduler")
	assert.Contains(t, controlPlaneServices, "controller-manager")
	assert.NotContains(t, controlPlaneServices, "etcd")

	// Etcd nodes own etcd but not the API server components.
	etcdServices := ops.DistroServices(capr.RuntimeRKE2, etcd)
	assert.Contains(t, etcdServices, "etcd")
	assert.NotContains(t, etcdServices, "scheduler")
}

// --- unsupportedServices ---------------------------------------------------------------------

func TestUnsupportedServices_RequestingRKE2ServiceOnK3sClusterFails(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{runtime: capr.RuntimeK3S}
	targets := []*corev1.Secret{
		certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"}),
		certificateRotationSecret(map[string]string{capr.WorkerRoleLabel: "true"}),
	}

	unsupported := unsupportedServices(adapter, []string{"rke2-server"}, targets)
	assert.Equal(t, []string{"rke2-server"}, unsupported)
}

func TestUnsupportedServices_RequestingK3sServiceOnRKE2ClusterFails(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}
	targets := []*corev1.Secret{
		certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"}),
		certificateRotationSecret(map[string]string{capr.WorkerRoleLabel: "true"}),
	}

	unsupported := unsupportedServices(adapter, []string{"k3s-server"}, targets)
	assert.Equal(t, []string{"k3s-server"}, unsupported)
}

func TestUnsupportedServices_EmptyServicesRetainsAllServiceBehavior(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}
	targets := []*corev1.Secret{
		certificateRotationSecret(map[string]string{capr.WorkerRoleLabel: "true"}),
	}

	assert.Empty(t, unsupportedServices(adapter, nil, targets))
}

func TestUnsupportedServices_SupportedServiceOnSomeTargetPasses(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{runtime: capr.RuntimeRKE2}
	targets := []*corev1.Secret{
		certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"}),
		certificateRotationSecret(map[string]string{capr.WorkerRoleLabel: "true"}),
	}

	// "scheduler" is only exposed by the control-plane target, but it is exposed by at
	// least one target, so the request as a whole is valid.
	assert.Empty(t, unsupportedServices(adapter, []string{"scheduler"}, targets))
}

func certificateRotationSecret(labels map[string]string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: labels}}
}

// --- reconcileRotate preflight -----------------------------------------------------------------

func TestReconcileRotate_UnsupportedServiceFailsBeforePlanAssignment(t *testing.T) {
	t.Parallel()

	cluster := &unstructured.Unstructured{}
	cluster.SetName("test")

	controlPlaneSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cp-1",
			Namespace: "fleet-default",
			UID:       "cp-1-uid",
			Labels: map[string]string{
				capr.ClusterNameLabel:      "test",
				capr.ControlPlaneRoleLabel: "true",
			},
		},
		Type: plan.SecretTypeMachinePlan,
	}

	ctrl := gomock.NewController(t)
	secrets := ctrlfake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
		sel, err := labels.Parse(opts.LabelSelector)
		if err != nil {
			return nil, err
		}
		if controlPlaneSecret.Namespace != ns || !sel.Matches(labels.Set(controlPlaneSecret.Labels)) {
			return &corev1.SecretList{}, nil
		}
		return &corev1.SecretList{Items: []corev1.Secret{*controlPlaneSecret}}, nil
	}).AnyTimes()

	// h.store is deliberately left nil. Preflight must fail and return before reaching
	// AssignPlan, so an accidental call into the nil store fails the test immediately
	// rather than silently succeeding.
	h := &handler{secrets: secrets}

	op := &opv1alpha1.CertificateRotation{
		ObjectMeta: metav1.ObjectMeta{UID: "operation"},
		Spec: opv1alpha1.CertificateRotationSpec{
			Args: opv1alpha1.CertificateRotationArgs{
				Services: []string{"rke2-server"},
			},
		},
	}

	s := &scope{
		op:         op,
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    &stubAdapter{runtime: capr.RuntimeK3S},
	}

	status := opv1alpha1.CertificateRotationStatus{}
	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.CertificateRotationStepRotate)

	got, err := h.reconcileRotate(s, status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.PreflightCheckFailedReason, opv1alpha1.FailedCondition.GetReason(&got))
	assert.Contains(t, opv1alpha1.FailedCondition.GetMessage(&got), "rke2-server")
}
