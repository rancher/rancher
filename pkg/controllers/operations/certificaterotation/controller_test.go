package certificaterotation

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	settingsCalls     []string
	dataDirErr        error
	pauseCalls        []bool
}

func (a *stubAdapter) RuntimeCommand() string { return a.runtime }
func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) (string, error) {
	return a.dataDir, a.dataDirErr
}
func (a *stubAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	return a.provisioningDir
}
func (a *stubAdapter) DistroManifestPaths(dataDir string) ops.ManifestPaths {
	return ops.DistroManifestPaths(a.RuntimeCommand(), dataDir)
}
func (a *stubAdapter) ComponentTLSSettings(_ *corev1.Secret, component string) (ops.ComponentTLSSettings, error) {
	a.settingsCalls = append(a.settingsCalls, component)
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
func (a *stubAdapter) PauseCluster(paused bool) error {
	a.pauseCalls = append(a.pauseCalls, paused)
	return nil
}
func (a *stubAdapter) ServerUnit() string { return a.runtime }
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
func (a *stubAdapter) KubectlPath(_ *corev1.Secret) (string, error) {
	return "", nil
}
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

type fakeBeaconClient struct {
	plancontrollers.BeaconClient

	statusUpdates []*planv1alpha1.Beacon
}

func (f *fakeBeaconClient) UpdateStatus(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.statusUpdates = append(f.statusUpdates, beacon.DeepCopy())
	return beacon, nil
}

type fakeDynamic struct {
	enqueues int
}

func (f *fakeDynamic) Get(schema.GroupVersionKind, string, string) (runtime.Object, error) {
	return nil, nil
}

func (f *fakeDynamic) Enqueue(schema.GroupVersionKind, string, string) error {
	f.enqueues++
	return nil
}

func terminalScope(ownerKey string, adapter *stubAdapter) *scope {
	cluster := &unstructured.Unstructured{}
	cluster.SetAPIVersion("provisioning.cattle.io/v1")
	cluster.SetKind("Cluster")
	return &scope{
		ownerKey: ownerKey,
		op:       &opv1alpha1.CertificateRotation{ObjectMeta: metav1.ObjectMeta{Name: "rotation", Namespace: "fleet-default"}},
		beacon: &planv1alpha1.Beacon{
			Status: planv1alpha1.BeaconStatus{Active: true, Owner: ownerKey},
		},
		clusterObj: cluster,
		adapter:    adapter,
	}
}

func TestTerminalHandler_OwningOperationUnpausesAndReleasesBeacon(t *testing.T) {
	t.Parallel()

	for _, terminal := range []struct {
		name         string
		handler      func(*handler, *scope, opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error)
		wantEnqueues int
	}{
		{"canceled", (*handler).handleCanceled, 0},
		{"failed", (*handler).handleFailed, 0},
		{"succeeded", (*handler).handleSucceeded, 1},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			adapter := &stubAdapter{}
			beacons := &fakeBeaconClient{}
			dynamic := &fakeDynamic{}
			h := &handler{beacons: beacons, dynamic: dynamic}
			s := terminalScope("certificate-rotation/fleet-default/rotation", adapter)

			_, err := terminal.handler(h, s, opv1alpha1.CertificateRotationStatus{})
			assert.NoError(t, err)
			assert.Equal(t, []bool{false}, adapter.pauseCalls)
			if assert.Len(t, beacons.statusUpdates, 1) {
				assert.Empty(t, beacons.statusUpdates[0].Status.Owner)
				assert.False(t, beacons.statusUpdates[0].Status.Active)
			}
			assert.Equal(t, terminal.wantEnqueues, dynamic.enqueues)
		})
	}
}

func TestTerminalHandler_NonOwnerDoesNotUnpauseOrReleaseBeacon(t *testing.T) {
	t.Parallel()

	for _, terminal := range []struct {
		name    string
		handler func(*handler, *scope, opv1alpha1.CertificateRotationStatus) (opv1alpha1.CertificateRotationStatus, error)
	}{
		{"canceled", (*handler).handleCanceled},
		{"failed", (*handler).handleFailed},
		{"succeeded", (*handler).handleSucceeded},
	} {
		t.Run(terminal.name, func(t *testing.T) {
			adapter := &stubAdapter{}
			beacons := &fakeBeaconClient{}
			dynamic := &fakeDynamic{}
			h := &handler{beacons: beacons, dynamic: dynamic}
			s := terminalScope("certificate-rotation/fleet-default/rotation", adapter)
			s.beacon.Status.Owner = "certificate-rotation/fleet-default/newer-rotation"

			_, err := terminal.handler(h, s, opv1alpha1.CertificateRotationStatus{})
			assert.NoError(t, err)
			assert.Empty(t, adapter.pauseCalls)
			assert.Empty(t, beacons.statusUpdates)
			assert.Zero(t, dynamic.enqueues)
		})
	}
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

			instructions, err := componentCertificateCleanupInstructions(s, &corev1.Secret{}, tt.services, adapter.dataDir, adapter.DistroManifestPaths(adapter.dataDir))
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

	instructions, err := componentCertificateCleanupInstructions(s, &corev1.Secret{}, nil, "/var/lib/rancher/rke2", s.adapter.DistroManifestPaths("/var/lib/rancher/rke2"))
	assert.Nil(t, instructions)
	assert.EqualError(t, err, "settings failed")
}

func TestComponentCertificateCleanupInstructions_SkipsUnselectedComponentBeforeReadingSettings(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{
		runtime:         capr.RuntimeRKE2,
		dataDir:         "/var/lib/rancher/rke2",
		provisioningDir: "/var/lib/rancher/capr",
		settingsErr:     errors.New("settings failed"),
	}
	s := &scope{
		op: &opv1alpha1.CertificateRotation{
			ObjectMeta: metav1.ObjectMeta{UID: "operation"},
		},
		adapter: adapter,
	}

	// Only "etcd" is requested, so neither controller-manager nor scheduler is selected.
	// Their TLS settings must never be fetched — otherwise the configured settingsErr would
	// surface here even though neither component is relevant to this request.
	instructions, err := componentCertificateCleanupInstructions(s, &corev1.Secret{}, []string{"etcd"}, adapter.dataDir, adapter.DistroManifestPaths(adapter.dataDir))
	assert.NoError(t, err)
	assert.Empty(t, instructions)
	assert.Empty(t, adapter.settingsCalls)
}

func TestWindowsIdempotentRestartInstructions_UsesPassedRuntime(t *testing.T) {
	t.Parallel()

	instructions := windowsIdempotentRestartInstructions("certificate-rotation/restart", "operation", capr.RuntimeK3S)
	assert.Len(t, instructions, 1)

	instr := instructions[0]
	assert.Equal(t, "powershell.exe", instr.Command)
	assert.Contains(t, instr.Args, windowsIdempotentScriptPath)
	assert.Contains(t, instr.Args, windowsIdempotencyRoot)
	assert.Contains(t, instr.Args, "restart-service")
	assert.Contains(t, instr.Args, capr.RuntimeK3S)

	// Same inputs must always produce the same idempotency name, so a retry recognizes an
	// already-applied instruction instead of re-running it.
	again := windowsIdempotentRestartInstructions("certificate-rotation/restart", "operation", capr.RuntimeK3S)
	assert.Equal(t, instr.Name, again[0].Name)

	// A different value (operation UID) must produce a different idempotency name, so a new
	// operation's plan is not mistaken for one already applied.
	other := windowsIdempotentRestartInstructions("certificate-rotation/restart", "other-operation", capr.RuntimeK3S)
	assert.NotEqual(t, instr.Name, other[0].Name)
}

func TestCertificateRotationRuntimeInstructions_CustomDataDirWithServices(t *testing.T) {
	t.Parallel()

	s := &scope{
		op: &opv1alpha1.CertificateRotation{
			ObjectMeta: metav1.ObjectMeta{UID: "operation"},
		},
		adapter: &stubAdapter{
			runtime:         capr.RuntimeRKE2,
			provisioningDir: "/var/lib/rancher/capr",
		},
	}
	secret := &corev1.Secret{}

	instructions := certificateRotationRuntimeInstructions(
		s, secret, "/custom/data-dir", []string{"etcd", "api-server"})
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

	s := &scope{
		op: &opv1alpha1.CertificateRotation{
			ObjectMeta: metav1.ObjectMeta{UID: "operation"},
		},
		adapter: &stubAdapter{
			runtime:         capr.RuntimeRKE2,
			provisioningDir: "/var/lib/rancher/capr",
		},
	}
	secret := &corev1.Secret{}

	instructions := certificateRotationRuntimeInstructions(
		s, secret, "/custom/data-dir", nil)
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

func TestManifestRemovalInstructions_DataDirWithSpacesIsNotInterpolated(t *testing.T) {
	t.Parallel()

	dataDir := "/var/lib/rancher/testing/certificate rotation"
	instructions := manifestRemovalInstructions("/var/lib/rancher/capr", "operation", ops.DistroManifestPaths(capr.RuntimeRKE2, dataDir))
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

func TestReconcileRotate_DataDirectoryErrorReturnsBeforePlanAssignment(t *testing.T) {
	t.Parallel()

	dataDirErr := errors.New("data directory unavailable")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cp-1", Namespace: "fleet-default",
			Labels: map[string]string{
				capr.ClusterNameLabel:      "test",
				capr.ControlPlaneRoleLabel: "true",
			},
		},
		Type: plan.SecretTypeMachinePlan,
	}
	ctrl := gomock.NewController(t)
	secrets := ctrlfake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(&corev1.SecretList{Items: []corev1.Secret{*secret}}, nil)

	cluster := &unstructured.Unstructured{}
	cluster.SetName("test")
	h := &handler{secrets: secrets}
	status := opv1alpha1.CertificateRotationStatus{}
	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.CertificateRotationStepRotate)

	got, err := h.reconcileRotate(&scope{
		op: &opv1alpha1.CertificateRotation{
			ObjectMeta: metav1.ObjectMeta{UID: "operation"},
		},
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter: &stubAdapter{
			runtime:    capr.RuntimeRKE2,
			dataDirErr: dataDirErr,
		},
	}, status)
	assert.ErrorIs(t, err, dataDirErr)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
}

func TestReconcileRotate_AssignedPlanCarriesOperationEnvOnce(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
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
	secrets.EXPECT().List(gomock.Any(), gomock.Any()).Return(&corev1.SecretList{Items: []corev1.Secret{*secret}}, nil)

	var assigned *corev1.Secret
	secrets.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
		assigned = s
		return s, nil
	})

	cluster := &unstructured.Unstructured{}
	cluster.SetName("test")

	h := &handler{secrets: secrets, store: plan.NewStore(secrets)}

	op := &opv1alpha1.CertificateRotation{
		ObjectMeta: metav1.ObjectMeta{UID: "operation-uid"},
	}

	status := opv1alpha1.CertificateRotationStatus{}
	status.SetPhase(opv1alpha1.OperationPhaseInProgress)
	status.SetStep(opv1alpha1.CertificateRotationStepRotate)

	s := &scope{
		op:         op,
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter: &stubAdapter{
			runtime:         capr.RuntimeRKE2,
			dataDir:         "/var/lib/rancher/rke2",
			provisioningDir: "/var/lib/rancher/capr",
		},
	}

	got, err := h.reconcileRotate(s, status)
	assert.NoError(t, err)
	assert.NotEqual(t, opv1alpha1.OperationPhaseFailed, got.Phase)

	if !assert.NotNil(t, assigned, "AssignPlan must have been called") {
		return
	}

	var assignedPlan plan.Plan
	assert.NoError(t, json.Unmarshal(assigned.Data["plan"], &assignedPlan))
	assert.NotEmpty(t, assignedPlan.OneTimeInstructions)

	wantEnv := fmt.Sprintf("CERTIFICATE_ROTATION_OPERATION_UID=%s", op.UID)
	for _, instr := range assignedPlan.OneTimeInstructions {
		count := 0
		for _, e := range instr.Env {
			if e == wantEnv {
				count++
			}
		}
		assert.Equal(t, 1, count, "instruction %q must carry the operation env exactly once", instr.Name)
	}
}
