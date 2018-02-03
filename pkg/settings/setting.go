package settings

var (
	settings = map[string]Setting{}
	provider Provider

	AgentImage           = newSetting("agent-image", "rancher/agent")
	CACerts              = newSetting("cacerts", "")
	EngineInstallURL     = newSetting("engine-install-url", "https://releases.rancher.com/install-docker/17.03.sh")
	EngineNewestVersion  = newSetting("engine-newest-version", "v17.03.0")
	EngineSupportedRange = newSetting("engine-supported-range", "~v17.03.0")
	MachineVersion       = newSetting("machine-version", "dev")
	HelmVersion          = newSetting("helm-version", "dev")
	ServerImage          = newSetting("server-image", "rancher/server")
	ServerVersion        = newSetting("server-version", "dev")
	TelemetryOpt         = newSetting("telemetry-opt", "")
	UIFeedBackForm       = newSetting("ui-feedback-form", "")
	UIIndex              = newSetting("ui-index", "https://releases.rancher.com/ui/latest2/index.html")
	UIPL                 = newSetting("ui-pl", "rancher")
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
