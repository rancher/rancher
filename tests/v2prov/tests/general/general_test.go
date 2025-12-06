// Package general contains general tests for v2prov integration testing.
// These tests verify system-level settings and agent version configurations.
package general

import (
	"os"
	"testing"

	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test_General_SystemAgentVersion verifies that the system-agent-version setting
// is properly configured and matches the expected CATTLE_SYSTEM_AGENT_VERSION
// environment variable.
func Test_General_SystemAgentVersion(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	setting, err := clients.Mgmt.Setting().Get("system-agent-version", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, setting.Value)
	assert.True(t, setting.Value == os.Getenv("CATTLE_SYSTEM_AGENT_VERSION"))
}

// Test_General_WinsAgentVersion verifies that the wins-agent-version setting
// is properly configured and matches the expected CATTLE_WINS_AGENT_VERSION
// environment variable. This is used for Windows agent deployments.
func Test_General_WinsAgentVersion(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	setting, err := clients.Mgmt.Setting().Get("wins-agent-version", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, setting.Value)
	assert.True(t, setting.Value == os.Getenv("CATTLE_WINS_AGENT_VERSION"))
}

// Test_General_CSIProxyAgentVersion verifies that the csi-proxy-agent-version setting
// is properly configured and matches the expected CATTLE_CSI_PROXY_AGENT_VERSION
// environment variable. This is used for Windows CSI driver proxy deployments.
func Test_General_CSIProxyAgentVersion(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	setting, err := clients.Mgmt.Setting().Get("csi-proxy-agent-version", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, setting.Value)
	assert.True(t, setting.Value == os.Getenv("CATTLE_CSI_PROXY_AGENT_VERSION"))
}
