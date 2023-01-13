package management

import (
	"fmt"

	"github.com/rancher/rancher/pkg/data/util"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var FeatureAppNS = util.FeatureAppNS

func addDefaultPodSecurityAdmissionConfigurationTemplates(management *config.ManagementContext) error {
	psapts := management.Management.PodSecurityAdmissionConfigurationTemplates("")
	templates := []*v3.PodSecurityAdmissionConfigurationTemplate{
		newPodSecurityAdmissionConfigurationTemplatePrivileged(),
		newPodSecurityAdmissionConfigurationTemplateRestricted(),
	}
	for _, template := range templates {
		if _, err := psapts.Create(template); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating default '%s' pod security admission configuration template: %w", template.Name, err)
		}
	}
	return nil
}

func newPodSecurityAdmissionConfigurationTemplateRestricted() *v3.PodSecurityAdmissionConfigurationTemplate {
	return &v3.PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "restricted",
		},
		Description: "The default restricted pod security admission configuration template",
		Configuration: v3.PodSecurityAdmissionConfigurationTemplateSpec{
			Defaults: v3.PodSecurityAdmissionConfigurationTemplateDefaults{
				Enforce:        "restricted",
				EnforceVersion: "latest",
				Audit:          "restricted",
				AuditVersion:   "latest",
				Warn:           "restricted",
				WarnVersion:    "latest",
			},
			Exemptions: v3.PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{},
				RuntimeClasses: []string{},
				Namespaces:     FeatureAppNS,
			},
		},
	}
}

func newPodSecurityAdmissionConfigurationTemplatePrivileged() *v3.PodSecurityAdmissionConfigurationTemplate {
	return &v3.PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "privileged",
		},
		Description: "The default privileged pod security admission configuration template",
		Configuration: v3.PodSecurityAdmissionConfigurationTemplateSpec{
			Defaults: v3.PodSecurityAdmissionConfigurationTemplateDefaults{
				Enforce:        "privileged",
				EnforceVersion: "latest",
				Audit:          "privileged",
				AuditVersion:   "latest",
				Warn:           "privileged",
				WarnVersion:    "latest",
			},
			Exemptions: v3.PodSecurityAdmissionConfigurationTemplateExemptions{
				Usernames:      []string{},
				RuntimeClasses: []string{},
				Namespaces:     []string{},
			},
		},
	}
}
