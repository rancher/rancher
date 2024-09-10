package general

import (
	"os"
	"testing"

	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
