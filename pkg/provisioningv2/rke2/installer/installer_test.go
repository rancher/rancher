package installer

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestInstaller_WindowsInstallScript(t *testing.T) {
	// arrange
	a := assert.New(t)

	// act
	script, err := WindowsInstallScript(context.TODO(), "test", []corev1.EnvVar{}, "localhost")

	// assert
	a.Nil(err)
	a.NotNil(script)
	a.Contains(string(script), "$env:CATTLE_TOKEN=\"test\"")
	a.Contains(string(script), "$env:CATTLE_ROLE_NONE=true")
	a.Contains(string(script), "$env:CATTLE_ROLE_NONE=true")
	a.Contains(string(script), "$env:CSI_PROXY_URL")
	a.Contains(string(script), "$env:CSI_PROXY_VERSION")
	a.Contains(string(script), "$env:CSI_PROXY_KUBELET_PATH")
}
