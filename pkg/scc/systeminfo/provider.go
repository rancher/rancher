package systeminfo

import (
	"github.com/google/uuid"
	"github.com/rancher/rancher/pkg/settings"
	coreVersion "github.com/rancher/rancher/pkg/version"
	"net/url"
)

type InfoProvider struct {
	RancherUuid uuid.UUID
	ClusterUuid uuid.UUID
}

func NewInfoProvider(rancherUuid uuid.UUID, clusterUuid uuid.UUID) *InfoProvider {

	return &InfoProvider{
		RancherUuid: rancherUuid,
		ClusterUuid: clusterUuid,
	}
}

// GetVersion returns a version number for the systeminfo provider
func (i *InfoProvider) GetVersion() string {
	var version string
	version = coreVersion.Version
	if coreVersion.IsDevBuild() {
		// TODO: maybe SCC devs can give us a static dev version?
		version = "2.10.3"
	}

	return version
}

// GetProductIdentifier returns a triple of product, version and architecture
// Rancher always returns "rancher" as product, and "unknown" as the architecture
func (i *InfoProvider) GetProductIdentifier() (string, string, string) {
	return "rancher", i.GetVersion(), "unknown"
}

// ServerHostname returns the hostname of the Rancher server URL
func ServerHostname() string {
	serverUrl := settings.ServerURL.Get()
	if serverUrl == "" {
		return ""
	}
	parsed, _ := url.Parse(serverUrl)
	return parsed.Host
}

func IsServerUrlReady() bool {
	return ServerHostname() != ""
}
