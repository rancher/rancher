package systeminfo

import (
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/rancher/rancher/pkg/settings"
	coreVersion "github.com/rancher/rancher/pkg/version"
)

func TestNewInfoProvider(t *testing.T) {
	// Test with dev build
	// infoProvider := NewInfoProvider(rancherUuid, clusterUuid)
	assert.Equal(t, "other", SCCSafeVersion())

	// Test with non-dev build
	coreVersion.Version = "1.9.0"
	defer func() { coreVersion.Version = "dev" }()
	// infoProvider = NewInfoProvider(rancherUuid, clusterUuid)
	assert.Equal(t, "1.9.0", SCCSafeVersion())

	// Test with no mock version
	// infoProvider = NewInfoProvider(rancherUuid, clusterUuid)
	assert.Equal(t, coreVersion.Version, SCCSafeVersion())
}

func TestGetProductIdentifier(t *testing.T) {
	rancherUuid := uuid.Parse(uuid.New())
	clusterUuid := uuid.Parse(uuid.New())

	infoProvider := NewInfoProvider(nil).SetUuids(rancherUuid, clusterUuid)
	product, version, architecture := infoProvider.GetProductIdentifier()
	assert.Equal(t, "rancher", product)
	// When in dev mode, the info provider has to "lie" in order to connect with SCC
	// however, when not in dev mode, the info provider should return the correct version
	if versionIsDevBuild() {
		assert.NotEqual(t, coreVersion.Version, version)
	} else {
		assert.Equal(t, coreVersion.Version, version)
	}
	assert.Equal(t, SCCSafeVersion(), version)
	assert.Equal(t, "unknown", architecture)
}

func TestServerHostname(t *testing.T) {
	originalUrl := settings.ServerURL.Get()
	_ = settings.ServerURL.Set("https://example.com")
	defer func() { _ = settings.ServerURL.Set(originalUrl) }()
	hostname := ServerHostname()
	if hostname != "example.com" {
		t.Errorf("Expected hostname to be example.com but got %s", hostname)
	}

	// Test with empty server URL
	_ = settings.ServerURL.Set("")
	hostname = ServerHostname()
	if hostname != "" {
		t.Errorf("Expected hostname to be blank but got %s", hostname)
	}
}
