package management

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
)

func addDefaultPodSecurityAdmissionConfigurationTemplates(management *config.ManagementContext) error {
	psapts := management.Management.PodSecurityAdmissionConfigurationTemplates("")
	templates := []*v3.PodSecurityAdmissionConfigurationTemplate{
		v3.NewPodSecurityAdmissionConfigurationTemplatePrivileged(),
		v3.NewPodSecurityAdmissionConfigurationTemplateRestricted(),
	}
	for _, template := range templates {
		if _, err := psapts.Create(template); err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating default '%s' pod security admission configuration template: %w", template.Name, err)
		}
	}
	return nil
}
