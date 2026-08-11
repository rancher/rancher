package operations

import (
	"testing"

	bootstrapv1beta2 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta2"
	controlplanev1beta2 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestComponentTLSSettingsExtraction_ImportedSource(t *testing.T) {
	t.Parallel()

	args := []string{
		"--kube-controller-manager-arg", "secure-port=10261",
		"--kube-controller-manager-arg", "tls-cert-file=/custom/kcm.crt",
		"--kube-controller-manager-arg", "tls-private-key-file=/custom/kcm.key",
	}

	got := componentTLSSettingsFromOuterArgs(args, KubeControllerManagerProbeName)
	assert.Equal(t, ComponentTLSSettings{
		SecurePort:        "10261",
		TLSCertFile:       "/custom/kcm.crt",
		TLSPrivateKeyFile: "/custom/kcm.key",
	}, got)
}

func TestComponentTLSSettingsExtraction_CAPRSource(t *testing.T) {
	t.Parallel()

	configValue := []any{
		"secure-port=10262",
		"tls-cert-file=/custom/ks.crt",
		"tls-private-key-file=/custom/ks.key",
	}

	got := componentTLSSettingsFromConfigArg(configValue)
	assert.Equal(t, ComponentTLSSettings{
		SecurePort:        "10262",
		TLSCertFile:       "/custom/ks.crt",
		TLSPrivateKeyFile: "/custom/ks.key",
	}, got)
}

func TestComponentTLSSettingsExtraction_CAPRKE2Source(t *testing.T) {
	t.Parallel()

	adapter := &CAPRKE2Adapter{
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
	}

	got, err := adapter.CertificateRotationComponentTLSSettings(nil, KubeSchedulerProbeName)
	assert.NoError(t, err)
	assert.Equal(t, ComponentTLSSettings{
		SecurePort:        "10262",
		TLSCertFile:       "/custom/ks.crt",
		TLSPrivateKeyFile: "/custom/ks.key",
	}, got)
}

func TestSecureProbeArguments_UsesExplicitCertWithoutKey(t *testing.T) {
	t.Parallel()

	settings := ComponentTLSSettings{
		TLSCertFile: "/custom/component.crt",
	}

	assert.False(t, settings.HasCompleteTLSConfig())
	assert.Equal(t, []string{
		"tls-cert-file=/custom/component.crt",
	}, secureProbeArguments(settings))
}
