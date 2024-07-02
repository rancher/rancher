package management

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
)

var FeatureAppNS = []string{
	"ingress-nginx",                   // This is for Ingress, not feature app
	"kube-system",                     // Harvester, vSphere CPI, vSphere CSI, RKE2 restricted PSA Config
	"cattle-system",                   // AKS/GKE/EKS Operator, Webhook, System Upgrade Controller
	"cattle-epinio-system",            // Epinio
	"cattle-fleet-system",             // Fleet
	"cattle-fleet-local-system",       // Fleet for the local cluster
	"longhorn-system",                 // Longhorn
	"cattle-neuvector-system",         // Neuvector
	"cattle-monitoring-system",        // Monitoring and Sub-charts
	"rancher-alerting-drivers",        // Alert Driver
	"cis-operator-system",             // CIS Benchmark, RKE2 restricted PSA Config
	"cattle-csp-adapter-system",       // CSP Adapter
	"cattle-externalip-system",        // External IP Webhook
	"cattle-gatekeeper-system",        // Gatekeeper
	"istio-system",                    // Istio and Sub-charts
	"cattle-istio-system",             // Kiali
	"cattle-logging-system",           // Logging
	"cattle-windows-gmsa-system",      // Windows GMSA
	"cattle-sriov-system",             // Sriov
	"cattle-ui-plugin-system",         // UI Plugin System
	"tigera-operator",                 // RKE2 restricted PSA Config, source: https://github.com/rancher/rke2/blob/34633dcc188d3a79744636fe21529ef6f5d64d71/pkg/rke2/psa.go#L58
	"cattle-provisioning-capi-system", // CAPI core controller manager
}

// addDefaultPodSecurityAdmissionConfigurationTemplates creates or updates the default PSACTs with the builtin templates.
func addDefaultPodSecurityAdmissionConfigurationTemplates(management *config.ManagementContext) error {
	psactClient := management.Management.PodSecurityAdmissionConfigurationTemplates("")
	templates := []*v3.PodSecurityAdmissionConfigurationTemplate{
		newPodSecurityAdmissionConfigurationTemplatePrivileged(),
		newPodSecurityAdmissionConfigurationTemplateRestricted(),
	}
	for _, t := range templates {
		if _, err := psactClient.Create(t); err != nil {
			if !errors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create default '%s' pod security admission configuration: %w", t.Name, err)
			}
			logrus.Tracef("updating default '%s' pod security admission configuration", t.Name)
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				// get the latest version of the object from the k8s API directly
				existing, err := psactClient.Get(t.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				// We'd like to preserve user's additions to the exemptions, meanwhile merging everything from
				// the built-in template into the existing psact for Rancher to work properly. It means that any
				// value that is still in the built-in template but removed by user will be added back.
				final := mergeExemptions(existing, t)
				if _, err = psactClient.Update(final); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// mergeExemptions returns a new pointer to PodSecurityAdmissionConfigurationTemplate which values are copied from
// the base but the exemptions field is the union set of the values from the base and the additional PSACT.
func mergeExemptions(base, additional *v3.PodSecurityAdmissionConfigurationTemplate) *v3.PodSecurityAdmissionConfigurationTemplate {
	if base == nil {
		return additional
	}
	if additional == nil {
		return base
	}
	a := base.Configuration.Exemptions
	b := additional.Configuration.Exemptions
	final := base.DeepCopy()
	final.Configuration.Exemptions.Usernames = sets.NewString(a.Usernames...).Insert(b.Usernames...).List()
	final.Configuration.Exemptions.Namespaces = sets.NewString(a.Namespaces...).Insert(b.Namespaces...).List()
	final.Configuration.Exemptions.RuntimeClasses = sets.NewString(a.RuntimeClasses...).Insert(b.RuntimeClasses...).List()
	return final
}

func newPodSecurityAdmissionConfigurationTemplateRestricted() *v3.PodSecurityAdmissionConfigurationTemplate {
	return &v3.PodSecurityAdmissionConfigurationTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rancher-restricted",
		},
		Description: "This is the built-in restricted Pod Security Admission Configuration Template. " +
			"It defines a heavily restricted policy, based on current Pod hardening best practices. " +
			"This policy contains namespace level exemptions for Rancher components.",
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
			Name: "rancher-privileged",
		},
		Description: "This is the built-in unrestricted Pod Security Admission Configuration Template. " +
			"It defines the most permissive PSS policy, allowing for known privilege escalations. " +
			"This policy contains no exemptions.",
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
