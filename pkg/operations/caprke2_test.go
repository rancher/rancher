package operations

import (
	"errors"
	"testing"

	bootstrapv1beta2 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta2"
	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	"github.com/rancher/rancher/pkg/capr"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// --- RuntimeService ---------------------------------------------------------------------------

func TestCAPRKE2Adapter_RuntimeService(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		secret *corev1.Secret
		want   string
	}{
		{"control-plane", newSecret(map[string]string{capr.ControlPlaneRoleLabel: "true"}), "rke2-server"},
		{"etcd", newSecret(map[string]string{capr.EtcdRoleLabel: "true"}), "rke2-server"},
		{"worker-only", newSecret(map[string]string{capr.WorkerRoleLabel: "true"}), "rke2-agent"},
	}

	a := &CAPRKE2Adapter{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := a.RuntimeService(tc.secret)
			assert.Equal(t, tc.want, got, "RuntimeService mismatch for %s", tc.name)
		})
	}
}

// --- ComponentTLSSettings ------------------------------------------------

func TestCAPRKE2Adapter_ComponentTLSSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		adapter   *CAPRKE2Adapter
		component string
		want      ComponentTLSSettings
	}{
		{
			name: "scheduler with complete TLS settings",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeScheduler: &bootstrapv1beta2.ComponentConfig{
								ExtraArgs: []string{
									"secure-port=10262",
									"tls-cert-file=/custom/ks.crt",
									"tls-private-key-file=/custom/ks.key",
								},
							},
						},
					},
				},
			},
			component: KubeSchedulerProbeName,
			want: ComponentTLSSettings{
				SecurePort:        "10262",
				TLSCertFile:       "/custom/ks.crt",
				TLSPrivateKeyFile: "/custom/ks.key",
			},
		},
		{
			name: "controller-manager with custom port only",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeControllerManager: &bootstrapv1beta2.ComponentConfig{
								ExtraArgs: []string{
									"secure-port=10261",
								},
							},
						},
					},
				},
			},
			component: KubeControllerManagerProbeName,
			want:      ComponentTLSSettings{SecurePort: "10261"},
		},
		{
			name: "cert-dir is ignored",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeScheduler: &bootstrapv1beta2.ComponentConfig{
								ExtraArgs: []string{
									"cert-dir=/custom",
									"secure-port=10262",
								},
							},
						},
					},
				},
			},
			component: KubeSchedulerProbeName,
			want:      ComponentTLSSettings{SecurePort: "10262"},
		},
		{
			name: "incomplete TLS pair",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeControllerManager: &bootstrapv1beta2.ComponentConfig{
								ExtraArgs: []string{
									"tls-cert-file=/custom/kcm.crt",
								},
							},
						},
					},
				},
			},
			component: KubeControllerManagerProbeName,
			want:      ComponentTLSSettings{TLSCertFile: "/custom/kcm.crt"},
		},
		{
			name: "unknown component returns empty",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeScheduler: &bootstrapv1beta2.ComponentConfig{
								ExtraArgs: []string{"secure-port=10262"},
							},
						},
					},
				},
			},
			component: "unknown-component",
		},
		{
			name: "nil component config returns empty",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeScheduler: nil,
						},
					},
				},
			},
			component: KubeSchedulerProbeName,
		},
		{
			name: "empty ExtraArgs returns empty",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeControllerManager: &bootstrapv1beta2.ComponentConfig{
								ExtraArgs: []string{},
							},
						},
					},
				},
			},
			component: KubeControllerManagerProbeName,
		},
		{
			name: "args without equals are skipped",
			adapter: &CAPRKE2Adapter{
				controlPlane: &controlplanev1beta2.RKE2ControlPlane{
					Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
						ServerConfig: controlplanev1beta2.RKE2ServerConfig{
							KubeScheduler: &bootstrapv1beta2.ComponentConfig{
								ExtraArgs: []string{
									"some-flag",
									"secure-port=10262",
								},
							},
						},
					},
				},
			},
			component: KubeSchedulerProbeName,
			want:      ComponentTLSSettings{SecurePort: "10262"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.adapter.ComponentTLSSettings(nil, tt.component)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)

			// Verify HasCompleteTLSConfig for the complete TLS settings test
			if tt.name == "scheduler with complete TLS settings" {
				assert.True(t, got.HasCompleteTLSConfig())
			}
			if tt.name == "incomplete TLS pair" {
				assert.False(t, got.HasCompleteTLSConfig())
			}
		})
	}
}

// --- extraArgsFor ---------------------------------------------------------------------------

func TestCAPRKE2Adapter_extraArgsFor(t *testing.T) {
	t.Parallel()

	adapter := &CAPRKE2Adapter{
		controlPlane: &controlplanev1beta2.RKE2ControlPlane{
			Spec: controlplanev1beta2.RKE2ControlPlaneSpec{
				ServerConfig: controlplanev1beta2.RKE2ServerConfig{
					KubeAPIServer: &bootstrapv1beta2.ComponentConfig{
						ExtraArgs: []string{"apiserver-arg=value"},
					},
					KubeControllerManager: &bootstrapv1beta2.ComponentConfig{
						ExtraArgs: []string{"kcm-arg=value"},
					},
					KubeScheduler: &bootstrapv1beta2.ComponentConfig{
						ExtraArgs: []string{"scheduler-arg=value"},
					},
				},
			},
		},
	}

	tests := []struct {
		name      string
		component string
		want      []string
	}{
		{
			name:      "kube-apiserver",
			component: KubeAPIServerProbeName,
			want:      []string{"apiserver-arg=value"},
		},
		{
			name:      "kube-controller-manager",
			component: KubeControllerManagerProbeName,
			want:      []string{"kcm-arg=value"},
		},
		{
			name:      "kube-scheduler",
			component: KubeSchedulerProbeName,
			want:      []string{"scheduler-arg=value"},
		},
		{
			name:      "unknown component",
			component: "unknown",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := adapter.extraArgsFor(tt.component)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- DistroDataDirectory ---------------------------------------------------------------------

func TestCAPRKE2Adapter_DistroDataDirectory_MachineLookupFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)
	machineCache.EXPECT().Get("fleet-default", "machine-a").Return(nil, errors.New("boom"))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "fleet-default",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "cluster.x-k8s.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "machine-a",
			},
		},
	}

	adapter := &CAPRKE2Adapter{
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				RESTMapper: &fakeRESTMapper{},
			},
			CAPI: &stubCAPIInterface{machineCache: machineCache},
		},
	}

	dir, err := adapter.DistroDataDirectory(secret)
	assert.Error(t, err)
	assert.Empty(t, dir)
	assert.NotEqual(t, "/var/lib/rancher/rke2", dir)
}

func TestCAPRKE2Adapter_DistroDataDirectory_IncompleteBootstrapConfigRef(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	machineCache := ctrlfake.NewMockCacheInterface[*capi.Machine](ctrl)
	machineCache.EXPECT().Get("fleet-default", "machine-a").Return(&capi.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: "machine-a", Namespace: "fleet-default"},
	}, nil)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-plan",
			Namespace: "fleet-default",
			Labels: map[string]string{
				planv1alpha1.MachineLifecycleGroupLabel: "cluster.x-k8s.io",
				planv1alpha1.MachineLifecycleKindLabel:  "Machine",
				planv1alpha1.MachineLifecycleNameLabel:  "machine-a",
			},
		},
	}

	adapter := &CAPRKE2Adapter{
		clients: &wrangler.CAPIContext{
			Context: &wrangler.Context{
				RESTMapper: &fakeRESTMapper{},
			},
			CAPI: &stubCAPIInterface{machineCache: machineCache},
		},
	}

	dir, err := adapter.DistroDataDirectory(secret)
	assert.Error(t, err)
	assert.Empty(t, dir)
	assert.NotEqual(t, "/var/lib/rancher/rke2", dir)
}
