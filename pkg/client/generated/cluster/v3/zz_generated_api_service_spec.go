package client

const (
	APIServiceSpecType                       = "apiServiceSpec"
	APIServiceSpecFieldCABundle              = "caBundle"
	APIServiceSpecFieldGroup                 = "group"
	APIServiceSpecFieldGroupPriorityMinimum  = "groupPriorityMinimum"
	APIServiceSpecFieldInsecureSkipTLSVerify = "insecureSkipTLSVerify"
	APIServiceSpecFieldService               = "service"
	APIServiceSpecFieldVersion               = "version"
	APIServiceSpecFieldVersionPriority       = "versionPriority"
)

type APIServiceSpec struct {
	CABundle              string            `json:"caBundle,omitempty" yaml:"caBundle,omitempty"`
	Group                 string            `json:"group,omitempty" yaml:"group,omitempty"`
	GroupPriorityMinimum  int64             `json:"groupPriorityMinimum,omitempty" yaml:"groupPriorityMinimum,omitempty"`
	InsecureSkipTLSVerify bool              `json:"insecureSkipTLSVerify,omitempty" yaml:"insecureSkipTLSVerify,omitempty"`
	Service               *ServiceReference `json:"service,omitempty" yaml:"service,omitempty"`
	Version               string            `json:"version,omitempty" yaml:"version,omitempty"`
	VersionPriority       int64             `json:"versionPriority,omitempty" yaml:"versionPriority,omitempty"`
}
