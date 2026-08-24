package operations

import (
	"testing"

	bootstrapv1beta2 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta2"
	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	"github.com/stretchr/testify/assert"
)

// --- CertificateRotationComponentTLSSettings ------------------------------------------------

func TestCAPRKE2Adapter_CertificateRotationComponentTLSSettings(t *testing.T) {
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
			got, err := tt.adapter.CertificateRotationComponentTLSSettings(nil, tt.component)
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
