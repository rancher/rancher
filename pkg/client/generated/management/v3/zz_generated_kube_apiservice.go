package client

const (
	KubeAPIServiceType                          = "kubeAPIService"
	KubeAPIServiceFieldAdmissionConfiguration   = "admissionConfiguration"
	KubeAPIServiceFieldAlwaysPullImages         = "alwaysPullImages"
	KubeAPIServiceFieldAuditLog                 = "auditLog"
	KubeAPIServiceFieldEventRateLimit           = "eventRateLimit"
	KubeAPIServiceFieldExtraArgs                = "extraArgs"
	KubeAPIServiceFieldExtraArgsArray           = "extraArgsArray"
	KubeAPIServiceFieldExtraBinds               = "extraBinds"
	KubeAPIServiceFieldExtraEnv                 = "extraEnv"
	KubeAPIServiceFieldImage                    = "image"
	KubeAPIServiceFieldPodSecurityConfiguration = "podSecurityConfiguration"
	KubeAPIServiceFieldPodSecurityPolicy        = "podSecurityPolicy"
	KubeAPIServiceFieldSecretsEncryptionConfig  = "secretsEncryptionConfig"
	KubeAPIServiceFieldServiceClusterIPRange    = "serviceClusterIpRange"
	KubeAPIServiceFieldServiceNodePortRange     = "serviceNodePortRange"
	KubeAPIServiceFieldWindowsExtraArgs         = "winExtraArgs"
	KubeAPIServiceFieldWindowsExtraArgsArray    = "winExtraArgsArray"
	KubeAPIServiceFieldWindowsExtraBinds        = "winExtraBinds"
	KubeAPIServiceFieldWindowsExtraEnv          = "winExtraEnv"
)

type KubeAPIService struct {
	AdmissionConfiguration   map[string]interface{}   `json:"admissionConfiguration,omitempty" yaml:"admissionConfiguration,omitempty"`
	AlwaysPullImages         bool                     `json:"alwaysPullImages,omitempty" yaml:"alwaysPullImages,omitempty"`
	AuditLog                 *AuditLog                `json:"auditLog,omitempty" yaml:"auditLog,omitempty"`
	EventRateLimit           *EventRateLimit          `json:"eventRateLimit,omitempty" yaml:"eventRateLimit,omitempty"`
	ExtraArgs                map[string]string        `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraArgsArray           map[string][]string      `json:"extraArgsArray,omitempty" yaml:"extraArgsArray,omitempty"`
	ExtraBinds               []string                 `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv                 []string                 `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	Image                    string                   `json:"image,omitempty" yaml:"image,omitempty"`
	PodSecurityConfiguration string                   `json:"podSecurityConfiguration,omitempty" yaml:"podSecurityConfiguration,omitempty"`
	PodSecurityPolicy        bool                     `json:"podSecurityPolicy,omitempty" yaml:"podSecurityPolicy,omitempty"`
	SecretsEncryptionConfig  *SecretsEncryptionConfig `json:"secretsEncryptionConfig,omitempty" yaml:"secretsEncryptionConfig,omitempty"`
	ServiceClusterIPRange    string                   `json:"serviceClusterIpRange,omitempty" yaml:"serviceClusterIpRange,omitempty"`
	ServiceNodePortRange     string                   `json:"serviceNodePortRange,omitempty" yaml:"serviceNodePortRange,omitempty"`
	WindowsExtraArgs         map[string]string        `json:"winExtraArgs,omitempty" yaml:"winExtraArgs,omitempty"`
	WindowsExtraArgsArray    map[string][]string      `json:"winExtraArgsArray,omitempty" yaml:"winExtraArgsArray,omitempty"`
	WindowsExtraBinds        []string                 `json:"winExtraBinds,omitempty" yaml:"winExtraBinds,omitempty"`
	WindowsExtraEnv          []string                 `json:"winExtraEnv,omitempty" yaml:"winExtraEnv,omitempty"`
}
