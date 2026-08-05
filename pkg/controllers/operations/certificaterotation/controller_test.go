package certificaterotation

import (
	"testing"

	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateCertificateRotationServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		services []string
		valid    bool
	}{
		{
			name:  "all services",
			valid: true,
		},
		{
			name:     "known services",
			services: []string{"etcd", "controller-manager", "scheduler"},
			valid:    true,
		},
		{
			name:     "unknown service",
			services: []string{"not-a-service"},
			valid:    false,
		},
		{
			name:     "known and unknown services",
			services: []string{"etcd", "not-a-service"},
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCertificateRotationServices(tt.services)
			if tt.valid {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, `unsupported certificate rotation service "not-a-service"`)
		})
	}
}

func TestComponentCertificateCleanupInstructions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		runtime                   string
		services                  []string
		controllerManagerSettings ops.ComponentTLSSettings
		schedulerSettings         ops.ComponentTLSSettings
		expected                  []string
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
			controllerManagerSettings: ops.ComponentTLSSettings{
				TLSCertFile:       "/custom/kcm.crt",
				TLSPrivateKeyFile: "/custom/kcm.key",
			},
		},
		{
			name:     "custom controller-manager TLS does not suppress scheduler cleanup",
			runtime:  capr.RuntimeRKE2,
			services: []string{"controller-manager", "scheduler"},
			controllerManagerSettings: ops.ComponentTLSSettings{
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
			instructions := componentCertificateCleanupInstructions("/var/lib/rancher/capr", "operation", tt.runtime, "/var/lib/rancher/"+tt.runtime, tt.services, tt.controllerManagerSettings, tt.schedulerSettings)
			assert.Len(t, instructions, len(tt.expected))
			for i, instruction := range instructions {
				assert.GreaterOrEqual(t, len(instruction.Args), 4)
				assert.Equal(t, "rm", instruction.Args[len(instruction.Args)-4])
				assert.Equal(t, []string{"-f", tt.expected[i]}, instruction.Args[len(instruction.Args)-2:])
			}
		})
	}
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
