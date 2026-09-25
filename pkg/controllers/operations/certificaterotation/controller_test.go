package certificaterotation

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// The four methods below complete the ops.Adapter contract for the restore-mode machinery. Nothing
// in certificate rotation restores cluster configuration or installs a distro version, so each is a
// no-op: RestoreTarget serves no resources, the update is never reached, the wait never blocks, and
// there is no version to install.
func (a *stubAdapter) RestoreTarget(_ string) (*unstructured.Unstructured, error) { return nil, nil }
func (a *stubAdapter) UpdateRestoreTarget(_ *unstructured.Unstructured) error     { return nil }
func (a *stubAdapter) WaitForRestoreTarget() (bool, error)                        { return true, nil }
func (a *stubAdapter) InstallInstruction(_ *corev1.Secret, _ string) (plan.OneTimeInstruction, bool) {
	return plan.OneTimeInstruction{}, false
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

	// beacon is what Get serves, and is kept in step with UpdateStatus so a handler driven over
	// several reconciles observes its own beacon writes. A nil beacon makes Get report NotFound.
	beacon *planv1alpha1.Beacon

	statusUpdates []*planv1alpha1.Beacon
	updates       []*planv1alpha1.Beacon
}

func (f *fakeBeaconClient) Get(namespace, name string, _ metav1.GetOptions) (*planv1alpha1.Beacon, error) {
	if f.beacon == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "beacons"}, namespace+"/"+name)
	}
	return f.beacon.DeepCopy(), nil
}

// Update models the main-resource endpoint: Beacon has a status subresource, so a status change
// sent here is silently dropped. Keeping the fake honest about that is what stops a handler which
// clears beacon ownership through Update from passing.
func (f *fakeBeaconClient) Update(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	updated := beacon.DeepCopy()
	if f.beacon != nil {
		updated.Status = f.beacon.Status
	}
	f.updates = append(f.updates, updated.DeepCopy())
	return updated, nil
}

func (f *fakeBeaconClient) UpdateStatus(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.statusUpdates = append(f.statusUpdates, beacon.DeepCopy())
	f.beacon = beacon.DeepCopy()
	return beacon, nil
}

type fakeDynamic struct {
	// getObj is served for a Get whose name matches it. Honoring the name is what lets a test point
	// an operation at a cluster which is not there: what the reconcile sees once a cluster has been
	// deleted underneath an operation.
	getObj runtime.Object

	enqueues int
}

func (f *fakeDynamic) Get(_ schema.GroupVersionKind, _, name string) (runtime.Object, error) {
	obj, ok := f.getObj.(metav1.Object)
	if !ok || obj.GetName() != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, name)
	}
	return f.getObj, nil
}

func (f *fakeDynamic) Enqueue(schema.GroupVersionKind, string, string) error {
	f.enqueues++
	return nil
}

type fakeCertificateRotationController struct {
	operationcontrollers.CertificateRotationController

	enqueueCalls int
	deleteCalls  int
	// updates records the objects passed to Update — the finalizer is the only thing the handler
	// writes outside of status, so each entry is a finalizer add or removal.
	updates []*opv1alpha1.CertificateRotation
}

func (f *fakeCertificateRotationController) EnqueueAfter(_, _ string, _ time.Duration) {
	f.enqueueCalls++
}

func (f *fakeCertificateRotationController) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	f.deleteCalls++
	return nil
}

func (f *fakeCertificateRotationController) Update(op *opv1alpha1.CertificateRotation) (*opv1alpha1.CertificateRotation, error) {
	f.updates = append(f.updates, op.DeepCopy())
	return op, nil
}

// testClusterGVK is a synthetic cluster kind registered with the ops adapter factory below, so the
// OnChange tests can drive a reconcile end to end — cluster lookup, adapter, beacon lookup, phase
// dispatch — without standing up a real provisioning/CAPI cluster. Using a dedicated kind also
// keeps the registration from shadowing the adapter of a kind that ships with Rancher.
var testClusterGVK = schema.GroupVersionKind{Group: "test.cattle.io", Version: "v1", Kind: "TestCertRotationCluster"}

func init() {
	ops.RegisterAdapter(testClusterGVK, func(_ *wrangler.CAPIContext, _ *unstructured.Unstructured) (ops.Adapter, error) {
		return &stubAdapter{runtime: capr.RuntimeRKE2}, nil
	})
}

func newOp() *opv1alpha1.CertificateRotation {
	return &opv1alpha1.CertificateRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "rotation-1",
			Namespace:  "fleet-default",
			UID:        "rotation-1-uid",
			Generation: 1,
		},
		Spec: opv1alpha1.CertificateRotationSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &corev1.ObjectReference{
					APIVersion: testClusterGVK.GroupVersion().String(),
					Kind:       testClusterGVK.Kind,
					Namespace:  "fleet-default",
					Name:       "test",
				},
			},
		},
	}
}

// newDeletingOp returns an operation which has been deleted and still carries our finalizer — the
// state the API server leaves an in-flight operation in until the controller releases it.
func newDeletingOp() *opv1alpha1.CertificateRotation {
	op := newOp()
	op.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	op.Finalizers = []string{Finalizer}
	return op
}

// missingClusterOp points an operation at a cluster the dynamic resolver does not serve, which is
// what the reconcile sees once a cluster has been deleted underneath an operation.
func missingClusterOp(op *opv1alpha1.CertificateRotation) *opv1alpha1.CertificateRotation {
	op.Spec.ClusterRef.Name = "gone"
	return op
}

func newBeacon(owner string, active bool) *planv1alpha1.Beacon {
	return &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "fleet-default"},
		Status:     planv1alpha1.BeaconStatus{Owner: owner, Active: active},
	}
}

// testOwnerKey is the beacon claim the controller computes for the canonical newOp().
var testOwnerKey = ops.BeaconOwnerKey(OperationKind, newOp())

// newOnChangeHandler wires a handler for the full OnChange path: the cluster referenced by newOp
// resolves through the dynamic resolver, and the beacon lookup serves (and mutates) the given
// beacon. A nil beacon makes the lookup report NotFound.
func newOnChangeHandler(beacon *planv1alpha1.Beacon) (*handler, *fakeCertificateRotationController, *fakeBeaconClient) {
	controller := &fakeCertificateRotationController{}
	beacons := &fakeBeaconClient{beacon: beacon}

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(testClusterGVK)
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test")

	h := &handler{
		certificateRotations: controller,
		beacons:              beacons,
		dynamic:              &fakeDynamic{getObj: cluster},
	}

	return h, controller, beacons
}

func terminalScope(adapter *stubAdapter) *scope {
	cluster := &unstructured.Unstructured{}
	cluster.SetAPIVersion("provisioning.cattle.io/v1")
	cluster.SetKind("Cluster")

	op := &opv1alpha1.CertificateRotation{
		ObjectMeta: metav1.ObjectMeta{Name: "rotation", Namespace: "fleet-default", UID: "rotation-uid"},
	}
	ownerKey := ops.BeaconOwnerKey(OperationKind, op)

	return &scope{
		ownerKey: ownerKey,
		op:       op,
		beacon: &planv1alpha1.Beacon{
			Status: planv1alpha1.BeaconStatus{Active: true, Owner: ownerKey},
		},
		clusterObj: cluster,
		adapter:    adapter,
	}
}

// Terminal handling releases the beacon but leaves the cluster paused: only a rotation which
// rotated every node has left the cluster fit to hand back to the provisioner, so finishRotation is
// the one place that unpauses. A rotation which ended any other way stays paused for an
// administrator, which is what TestFinishRotation covers from the other side.
func TestTerminalHandler_OwningOperationReleasesBeaconAndLeavesClusterPaused(t *testing.T) {
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
			s := terminalScope(adapter)

			got, err := terminal.handler(h, s, opv1alpha1.CertificateRotationStatus{})
			require.NoError(t, err)
			assert.Empty(t, adapter.pauseCalls, "terminal handling must not unpause the cluster")
			require.Len(t, beacons.statusUpdates, 1)
			assert.Empty(t, beacons.statusUpdates[0].Status.Owner)
			assert.False(t, beacons.statusUpdates[0].Status.Active)
			assert.False(t, got.TerminatedAt.IsZero(), "the beacon was released, so terminal handling is complete")

			assert.Equal(t, terminal.wantEnqueues, dynamic.enqueues)
		})
	}
}

func TestTerminalHandler_NonOwnerLeavesBeaconUntouched(t *testing.T) {
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
			s := terminalScope(adapter)
			s.beacon.Status.Owner = ops.BeaconOwnerKey(OperationKind, &opv1alpha1.CertificateRotation{
				ObjectMeta: metav1.ObjectMeta{Name: "rotation", Namespace: "fleet-default", UID: "newer-rotation-uid"},
			})

			_, err := terminal.handler(h, s, opv1alpha1.CertificateRotationStatus{})
			require.NoError(t, err)
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
			require.NoError(t, err)
			require.Len(t, instructions, len(tt.expected))
			for i, instruction := range instructions {
				require.GreaterOrEqual(t, len(instruction.Args), 4)
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
	require.NoError(t, err)
	assert.Empty(t, instructions)
	assert.Empty(t, adapter.settingsCalls)
}

func TestWindowsIdempotentRestartInstructions_UsesPassedRuntime(t *testing.T) {
	t.Parallel()

	instructions := windowsIdempotentRestartInstructions("certificate-rotation/restart", "operation", capr.RuntimeK3S)
	require.Len(t, instructions, 1)

	instr := instructions[0]
	assert.Equal(t, "powershell.exe", instr.Command)
	assert.Equal(t, []string{
		windowsIdempotentScriptPath,
		"certificate-rotation/restart-restart",
		plan.PlanHash([]byte("operation")),
		plan.PlanHash([]byte("restart-service")),
		"restart-service",
		windowsIdempotencyRoot,
		capr.RuntimeK3S,
	}, instr.Args)

	// Same inputs must always produce the same idempotency name, so a retry recognizes an
	// already-applied instruction instead of re-running it.
	again := windowsIdempotentRestartInstructions("certificate-rotation/restart", "operation", capr.RuntimeK3S)
	require.Len(t, again, 1)
	assert.Equal(t, instr.Name, again[0].Name)

	// A different value (operation UID) must produce a different idempotency name, so a new
	// operation's plan is not mistaken for one already applied.
	other := windowsIdempotentRestartInstructions("certificate-rotation/restart", "other-operation", capr.RuntimeK3S)
	require.Len(t, other, 1)
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
	require.Len(t, instructions, 1)

	args := instructions[0].Args
	require.GreaterOrEqual(t, len(args), 8)
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
	require.Len(t, instructions, 1)

	args := instructions[0].Args
	require.GreaterOrEqual(t, len(args), 4)
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
	require.Len(t, instructions, 1)

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

	adapter := s.adapter.(*stubAdapter)

	got, err := h.reconcileRotate(s, status)
	require.NoError(t, err)
	// The operation called its own work off before dispatching any, which is Aborted rather than
	// Failed: nothing was attempted and lost.
	assert.Equal(t, opv1alpha1.OperationPhaseAborted, got.Phase)
	assert.Equal(t, opv1alpha1.PreflightCheckFailedReason, opv1alpha1.AbortedCondition.GetReason(&got))
	assert.Contains(t, opv1alpha1.AbortedCondition.GetMessage(&got), "rke2-server")
	assert.Empty(t, adapter.pauseCalls, "a request rejected before any plan is assigned must not pause the cluster")
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
	require.NoError(t, err)
	assert.NotEqual(t, opv1alpha1.OperationPhaseFailed, got.Phase)

	if !assert.NotNil(t, assigned, "AssignPlan must have been called") {
		return
	}

	var assignedPlan plan.Plan
	require.NoError(t, json.Unmarshal(assigned.Data["plan"], &assignedPlan))
	require.NotEmpty(t, assignedPlan.OneTimeInstructions)

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

// --- the reconcile: pausing, deletion, cancellation and a cluster or beacon that is not there ----

func TestOnChange_Paused(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Paused = true
	op.Status = opv1alpha1.CertificateRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.CertificateRotationStepRotate,
	}

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, status.Phase, "a paused operation stands still")
	assert.Equal(t, "True", opv1alpha1.PausedCondition.GetStatus(&status), "its conditions are still refreshed")
	assert.Empty(t, beacons.statusUpdates, "the beacon must be left exactly as the pause found it")
	assert.Empty(t, controller.updates, "a paused operation is not finalized, so it takes no finalizer")
	assert.Zero(t, controller.enqueueCalls, "a paused operation is not polled")
}

func TestOnChange_TakesFinalizer(t *testing.T) {
	t.Parallel()

	// Pending keeps the reconcile in handlePending rather than dispatching plans the fake has no
	// secrets for.
	op := newOp()
	op.Status = opv1alpha1.CertificateRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhasePending},
	}

	h, controller, _ := newOnChangeHandler(newBeacon("", false))

	_, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	if assert.Len(t, controller.updates, 1, "an operation in flight must be finalized") {
		assert.Contains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestOnChange_DeletionCancelsInFlightOperation covers the core of the deletion contract: an
// operation deleted while it is still running is canceled, its beacon is released, and only then is
// the finalizer retired — one reconcile later, so the canceled status is persisted for observers
// before the object is allowed to disappear.
func TestOnChange_DeletionCancelsInFlightOperation(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Status = opv1alpha1.CertificateRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.CertificateRotationStepRotate,
	}

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.Equal(t, opv1alpha1.OperationDeletedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.False(t, status.TerminatedAt.IsZero(), "handleCanceled ran to completion, so it must be recorded")
	if assert.Len(t, beacons.statusUpdates, 1, "the beacon must be released on the way out") {
		assert.Empty(t, beacons.statusUpdates[0].Status.Owner)
	}
	assert.Empty(t, controller.updates, "the finalizer must outlive the status write that records the cancellation")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	require.NoError(t, err)
	if assert.Len(t, controller.updates, 1) {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestOnChange_DeletionOfPausedOperation covers a paused operation being deleted. Pausing stops the
// controller touching the operation at all, and tearing it down is no exception: the deletion waits
// for the pause to lift.
func TestOnChange_DeletionOfPausedOperation(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Spec.Paused = true
	op.Status = opv1alpha1.CertificateRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.CertificateRotationStepRotate,
	}

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, status.Phase, "a paused operation is not reconciled, even to cancel it")
	assert.Empty(t, beacons.statusUpdates, "the beacon must be left exactly as the pause found it")
	assert.Empty(t, controller.updates, "the finalizer must stay until the operation is resumed")
}

// TestOnChange_CancelRequestedCancelsInFlightOperation mirrors the deletion contract for a
// cancellation requested through the spec.
func TestOnChange_CancelRequestedCancelsInFlightOperation(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Cancel = true
	op.Status = opv1alpha1.CertificateRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.CertificateRotationStepRotate,
	}

	h, _, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.Equal(t, opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.False(t, status.TerminatedAt.IsZero())
	assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(&status))
	if assert.Len(t, beacons.statusUpdates, 1, "the beacon must be released") {
		assert.Empty(t, beacons.statusUpdates[0].Status.Owner)
	}
}

// A cancellation requested after the operation concluded is declined and reported, rather than
// rewriting the phase it ended in.
func TestOnChange_CancelRequestedInTerminalPhaseIsDeclined(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Cancel = true
	op.Spec.TTL = -1

	initial := opv1alpha1.CertificateRotationStatus{Step: opv1alpha1.CertificateRotationStepRotate}
	initial.MarkSucceeded()
	initial.SetTerminated()
	op.Status = initial

	h, _, beacons := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase, "the phase it ended in stands")
	assert.Equal(t, "False", opv1alpha1.CanceledCondition.GetStatus(&status))
	assert.Equal(t, opv1alpha1.CancellationDeclinedReason, opv1alpha1.CanceledCondition.GetReason(&status),
		"a declined cancellation must be acknowledged, not silently passed over")
	assert.Empty(t, beacons.statusUpdates, "there is nothing left to release")
}

// TestOnChange_MissingClusterKeepsConcludedOutcome covers the rule that a missing cluster is only a
// failure for an operation which still has work to dispatch.
func TestOnChange_MissingClusterKeepsConcludedOutcome(t *testing.T) {
	t.Parallel()

	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseAborted,
		opv1alpha1.OperationPhaseCanceled,
	} {
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			op := missingClusterOp(newOp())
			op.Spec.TTL = -1

			initial := opv1alpha1.CertificateRotationStatus{Step: opv1alpha1.CertificateRotationStepRotate}
			initial.SetPhase(phase)
			op.Status = initial

			h, _, _ := newOnChangeHandler(newBeacon("", false))

			status, err := h.OnChange(op, op.Status)
			require.NoError(t, err)
			assert.Equal(t, phase, status.Phase, "the phase the operation ended in must stand")
			assert.False(t, status.TerminatedAt.IsZero(), "with no cluster there is nothing to release")
		})
	}
}

// The other half of that rule: a terminal phase hook which is still owed keeps the operation from
// recording termination, because its delegate has not had its turn.
func TestOnChange_MissingClusterWithOwedHookDefersTermination(t *testing.T) {
	t.Parallel()

	op := missingClusterOp(newOp())
	op.Spec.TTL = -1
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "verify": "delegate-a"}

	initial := opv1alpha1.CertificateRotationStatus{Step: opv1alpha1.CertificateRotationStepRotate}
	initial.MarkSucceeded()
	op.Status = initial

	h, _, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase)
	assert.True(t, status.TerminatedAt.IsZero(), "the delegate has not had its turn, so nothing may be recorded")
	assert.Equal(t, opv1alpha1.WaitingForDelegateReason, opv1alpha1.FinalizedCondition.GetReason(&status))
}

// TestOnChange_MissingClusterStillFailsRunningOperation guards that rule from over-reaching.
func TestOnChange_MissingClusterStillFailsRunningOperation(t *testing.T) {
	t.Parallel()

	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhasePending,
		opv1alpha1.OperationPhaseInProgress,
	} {
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			op := missingClusterOp(newOp())
			op.Spec.TTL = -1
			op.Status = opv1alpha1.CertificateRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
				Step:            opv1alpha1.CertificateRotationStepRotate,
			}

			h, _, _ := newOnChangeHandler(newBeacon("", false))

			status, err := h.OnChange(op, op.Status)
			require.NoError(t, err)
			assert.Equal(t, opv1alpha1.OperationPhaseFailed, status.Phase)
			assert.Equal(t, opv1alpha1.ClusterNotFoundReason, opv1alpha1.FailedCondition.GetReason(&status))
			assert.False(t, status.TerminatedAt.IsZero(),
				"there is no beacon to release, so the failure must not be left uncollectable")
		})
	}
}

// TestOnChange_MissingBeaconDisposition covers what an operation does when the beacon it needs is
// not there. What it should do depends entirely on what it still owes: an outcome which dispatched
// work owes a release and complains that it cannot make one, while an outcome which dispatched
// nothing owes nothing and finishes.
func TestOnChange_MissingBeaconDisposition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		phase      opv1alpha1.OperationPhase
		terminated bool

		wantErr        bool
		wantPhase      opv1alpha1.OperationPhase
		wantReason     string
		wantTerminated bool
	}{
		{
			name:           "aborted finishes",
			phase:          opv1alpha1.OperationPhaseAborted,
			wantPhase:      opv1alpha1.OperationPhaseAborted,
			wantTerminated: true,
		},
		{
			name:           "canceled finishes",
			phase:          opv1alpha1.OperationPhaseCanceled,
			wantPhase:      opv1alpha1.OperationPhaseCanceled,
			wantTerminated: true,
		},
		{
			name:      "succeeded complains and sticks",
			phase:     opv1alpha1.OperationPhaseSucceeded,
			wantErr:   true,
			wantPhase: opv1alpha1.OperationPhaseSucceeded,
		},
		{
			name:      "failed complains and sticks",
			phase:     opv1alpha1.OperationPhaseFailed,
			wantErr:   true,
			wantPhase: opv1alpha1.OperationPhaseFailed,
		},
		{
			name:           "succeeded and already terminated settles",
			phase:          opv1alpha1.OperationPhaseSucceeded,
			terminated:     true,
			wantPhase:      opv1alpha1.OperationPhaseSucceeded,
			wantTerminated: true,
		},
		{
			name:           "in progress fails",
			phase:          opv1alpha1.OperationPhaseInProgress,
			wantPhase:      opv1alpha1.OperationPhaseFailed,
			wantReason:     opv1alpha1.BeaconLostReason,
			wantTerminated: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			op := newOp()
			op.Spec.TTL = -1

			initial := opv1alpha1.CertificateRotationStatus{Step: opv1alpha1.CertificateRotationStepRotate}
			initial.SetPhase(tc.phase)
			if tc.terminated {
				initial.SetTerminated()
			}
			op.Status = initial

			h, _, _ := newOnChangeHandler(nil)

			status, err := h.OnChange(op, op.Status)
			if tc.wantErr {
				assert.Error(t, err, "the operation must complain about a beacon it cannot release")
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tc.wantPhase, status.Phase)
			if tc.wantReason != "" {
				outcome, _ := opv1alpha1.OutcomeConditionFor(status.Phase)
				assert.Equal(t, tc.wantReason, outcome.GetReason(&status))
			}
			assert.Equal(t, tc.wantTerminated, !status.TerminatedAt.IsZero(),
				"termination is recorded only when nothing is owed")
		})
	}
}

// A Pending operation has not acquired the beacon yet, so its absence is "not created" rather than
// "lost" and the operation waits.
func TestOnChange_MissingBeaconWhilePendingWaits(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.TTL = -1
	op.Status = opv1alpha1.CertificateRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhasePending},
	}

	h, _, _ := newOnChangeHandler(nil)

	status, err := h.OnChange(op, op.Status)
	require.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhasePending, status.Phase)
	assert.Equal(t, opv1alpha1.WaitingForBeaconReason, opv1alpha1.PendingCondition.GetReason(&status))
	assert.True(t, status.TerminatedAt.IsZero())
}

// TestOnChange_ExpiredTerminalOperationIsCollectedOnceTerminated pins that a settled operation is
// garbage collected only once the controller has finished with it.
func TestOnChange_ExpiredTerminalOperationIsCollectedOnceTerminated(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.TTL = 0

	initial := opv1alpha1.CertificateRotationStatus{Step: opv1alpha1.CertificateRotationStepRotate}
	initial.MarkSucceeded()
	initial.SetTerminated()
	op.Status = updateStatus(op, initial)

	h, controller, _ := newOnChangeHandler(newBeacon("", false))

	_, err := h.OnChange(op, op.Status)
	assert.ErrorIs(t, err, generic.ErrSkip, "a deleted operation reports ErrSkip so the status update is dropped")
	assert.Equal(t, 1, controller.deleteCalls)
}

// TestHandlePending_ReclaimsSupersededClaim covers the wiring of the no-lookup beacon reclaim.
func TestHandlePending_ReclaimsSupersededClaim(t *testing.T) {
	t.Parallel()

	op := newOp()
	superseded := ops.BeaconOwnerKey(OperationKind, &opv1alpha1.CertificateRotation{
		ObjectMeta: metav1.ObjectMeta{Namespace: op.Namespace, Name: op.Name, UID: "dead-uid"},
	})

	beacon := newBeacon(superseded, true)
	beacon.Status.Delegates = []string{"dead-delegate"}

	beacons := &fakeBeaconClient{beacon: beacon}
	h := &handler{beacons: beacons}
	s := &scope{
		ownerKey: ops.BeaconOwnerKey(OperationKind, op),
		op:       op,
		beacon:   beacon,
		adapter:  &stubAdapter{runtime: capr.RuntimeRKE2},
	}

	_, err := h.handlePending(s, opv1alpha1.CertificateRotationStatus{})
	require.NoError(t, err)
	assert.Equal(t, testOwnerKey, s.beacon.Status.Owner, "the operation must end up holding the beacon")
	assert.Empty(t, s.beacon.Status.Delegates, "the dead claim's delegate must not be inherited")
}

// TestFinishRotation covers the one place a rotation unpauses the cluster: the unpause and the
// success marker are done together, so a rotation cannot report success while leaving the cluster
// frozen, nor unpause without having succeeded.
func TestFinishRotation(t *testing.T) {
	t.Parallel()

	adapter := &stubAdapter{}
	h := &handler{}
	s := terminalScope(adapter)

	status, err := h.finishRotation(s, opv1alpha1.CertificateRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.CertificateRotationStepRotate,
	})
	require.NoError(t, err)
	assert.Equal(t, []bool{false}, adapter.pauseCalls, "exactly one unpause")
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase)
	assert.Equal(t, "True", opv1alpha1.SucceededCondition.GetStatus(&status))
}
