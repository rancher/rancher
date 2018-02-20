package settings

import (
	"encoding/json"

	"github.com/rancher/types/apis/management.cattle.io/v3"
)

var (
	settings = map[string]Setting{}
	provider Provider

	AgentImage                      = newSetting("agent-image", "rancher/agent:v2.0.3-rc3")
	CACerts                         = newSetting("cacerts", "")
	EngineInstallURL                = newSetting("engine-install-url", "https://releases.rancher.com/install-docker/17.03.sh")
	EngineNewestVersion             = newSetting("engine-newest-version", "v17.03.0")
	EngineSupportedRange            = newSetting("engine-supported-range", "~v17.03.0")
	HelmVersion                     = newSetting("helm-version", "dev")
	KubernetesVersion               = newSetting("k8s-version", "v1.8.7-rancher1-1")
	KubernetesVersionToSystemImages = newSetting("k8s-version-to-images", getSystemImages())
	MachineVersion                  = newSetting("machine-version", "dev")
	ServerImage                     = newSetting("server-image", "rancher/server")
	ServerVersion                   = newSetting("server-version", "dev")
	TelemetryOpt                    = newSetting("telemetry-opt", "")
	UIFeedBackForm                  = newSetting("ui-feedback-form", "")
	UIIndex                         = newSetting("ui-index", "https://releases.rancher.com/ui/latest2/index.html")
	UIPath                          = newSetting("ui-path", "")
	UIPL                            = newSetting("ui-pl", "rancher")
)

type Provider interface {
	Get(name string) string
	Set(name, value string) error
	SetAll(settings map[string]Setting) error
}

type Setting struct {
	Name     string
	Default  string
	ReadOnly bool
}

func (s Setting) Set(value string) error {
	if provider == nil {
		s, ok := settings[s.Name]
		if ok {
			s.Default = value
			settings[s.Name] = s
		}
	} else {
		return provider.Set(s.Name, value)
	}
	return nil
}

func (s Setting) Get() string {
	if provider == nil {
		s := settings[s.Name]
		return s.Default
	}
	return provider.Get(s.Name)
}

func SetProvider(p Provider) error {
	if err := p.SetAll(settings); err != nil {
		return err
	}
	provider = p
	return nil
}

func newSetting(name, def string) Setting {
	s := Setting{
		Name:    name,
		Default: def,
	}
	settings[s.Name] = s
	return s
}

func getSystemImages() string {
	versionToSystemImages := v3.K8sVersionToRKESystemImages

	data, err := json.Marshal(versionToSystemImages)
	if err != nil {
		return ""
	}
	return string(data)
}
