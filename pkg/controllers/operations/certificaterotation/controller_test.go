package certificaterotation

import (
	"errors"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
func (a *stubAdapter) ConfigFile(_ *corev1.Secret) string                 { return "" }
func (a *stubAdapter) ConfigDirectory(_ *corev1.Secret) string            { return "" }
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
					Spec: opv1alpha1.CertificateRotationSpec{
						Args: opv1alpha1.CertificateRotationArgs{
							Services: tt.services,
						},
					},
				},
				adapter: adapter,
			}

			instructions, err := componentCertificateCleanupInstructions(s, &corev1.Secret{})
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

	instructions, err := componentCertificateCleanupInstructions(s, &corev1.Secret{})
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

func TestServicesApply(t *testing.T) {
	t.Parallel()

	controlPlane := certificateRotationSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"})
	etcd := certificateRotationSecret(map[string]string{capr.EtcdRoleLabel: "true"})
	worker := certificateRotationSecret(map[string]string{capr.WorkerRoleLabel: "true"})

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
			assert.Equal(t, tt.want, servicesApply(tt.services, tt.secret))
		})
	}
}

func certificateRotationSecret(labels map[string]string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: labels}}
}
