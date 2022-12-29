package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type PodSecurityAdmissionConfigurationTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description   string                                        `json:"description"`
	Configuration PodSecurityAdmissionConfigurationTemplateSpec `json:"configuration"`
}

type PodSecurityAdmissionConfigurationTemplateSpec struct {
	Defaults   PodSecurityAdmissionConfigurationTemplateDefaults   `json:"defaults"`
	Exemptions PodSecurityAdmissionConfigurationTemplateExemptions `json:"exemptions,omitempty"`
}

// PodSecurityAdmissionConfigurationTemplateDefaults are applied when a mode label is not set.
//
// The level label values must be one of
// 'privileged' (default), 'baseline', or 'restricted'
//
// The version levels must be either 'latest' (default), or a specific version (e.g. 'v1.25')
type PodSecurityAdmissionConfigurationTemplateDefaults struct {
	Enforce        string `json:"enforce"`
	EnforceVersion string `json:"enforce-version" yaml:"enforce-version"`
	Audit          string `json:"audit"`
	AuditVersion   string `json:"audit-version" yaml:"audit-version"`
	Warn           string `json:"warn"`
	WarnVersion    string `json:"warn-version" yaml:"warn-version"`
}

type PodSecurityAdmissionConfigurationTemplateExemptions struct {
	Usernames      []string `json:"usernames"`
	RuntimeClasses []string `json:"runtimeClasses"`
	Namespaces     []string `json:"namespaces"`
}

func NewPodSecurityAdmissionConfigurationTemplateRestricted() *PodSecurityAdmissionConfigurationTemplate {
	return &PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "restricted",
		},
		Description: "The default restricted pod security admission configuration template",
		Configuration: PodSecurityAdmissionConfigurationTemplateSpec{
			Defaults: PodSecurityAdmissionConfigurationTemplateDefaults{
				Enforce:        "restricted",
				EnforceVersion: "latest",
				Audit:          "restricted",
				AuditVersion:   "latest",
				Warn:           "restricted",
				WarnVersion:    "latest",
			},
			Exemptions: PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{},
				RuntimeClasses: []string{},
				Namespaces:     []string{"ingress-nginx", "kube-system"},
			},
		},
	}
}

func NewPodSecurityAdmissionConfigurationTemplatePrivileged() *PodSecurityAdmissionConfigurationTemplate {
	return &PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "privileged",
		},
		Description: "The default privileged pod security admission configuration template",
		Configuration: PodSecurityAdmissionConfigurationTemplateSpec{
			Defaults: PodSecurityAdmissionConfigurationTemplateDefaults{
				Enforce:        "privileged",
				EnforceVersion: "latest",
				Audit:          "privileged",
				AuditVersion:   "latest",
				Warn:           "privileged",
				WarnVersion:    "latest",
			},
			Exemptions: PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{},
				RuntimeClasses: []string{},
				Namespaces:     []string{},
			},
		},
	}
}
