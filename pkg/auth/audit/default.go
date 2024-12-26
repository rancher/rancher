package audit

import (
	"strings"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	managementdata "github.com/rancher/rancher/pkg/data/management"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	sensitiveHeaders = []string{
		// Request Headers
		"Cookie", "Authorization", "X-Api-Tunnel-Params", "X-Api-Tunnel-Token", "X-Api-Auth-Header", "X-Amz-Security-Token",

		// Response Headers
		"Cookie", "Set-Cookie", "X-Api-Set-Cookie-Header",
	}

	sensitiveBodyFields = []string{"[pP]assword", "[tT]oken", "[kKube][cC]onfig", "credentials", "applicationSecret", "oauthCredential", "serviceAccountCredential", "spKey", "spCert", "certificate", "privateKey"}
)

func init() {
	for _, v := range managementdata.DriverData {
		for key, value := range v {
			if strings.HasPrefix(key, "public") || strings.HasPrefix(key, "optional") {
				continue
			}

			sensitiveBodyFields = append(sensitiveBodyFields, value...)
		}
	}
}

// DefaultPolicy is the policy deployed by default by rancher and cannot currently be disabled once it is installed.
func DefaultPolicies() []auditlogv1.AuditLogPolicy {
	return []auditlogv1.AuditLogPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redact-generic-info",
			},
			Spec: auditlogv1.AuditLogPolicySpec{
				Filters: []auditlogv1.Filter{
					{
						Action:     auditlogv1.FilterActionAllow,
						RequestURI: ".*",
					},
				},
				AdditionalRedactions: []auditlogv1.Redaction{
					{
						Headers: []string{
							strings.Join(sensitiveHeaders, "|"),
						},
						Keys: []string{
							strings.Join(sensitiveBodyFields, "|"),
						},
					},
				},
			},
		},
		// {
		// 	ObjectMeta: metav1.ObjectMeta{
		// 		Name: "redact-secrets",
		// 	},
		// 	Spec: auditlogv1.AuditLogPolicySpec{
		// 		Filters: []auditlogv1.Filter{
		// 			{
		// 				Action:     auditlogv1.FilterActionAllow,
		// 				RequestURI: ".*secrets.*",
		// 			},
		// 		},
		// 		AdditionalRedactions: []auditlogv1.Redaction{
		// 			{
		// 				Keys: []string{
		// 					"^(string)?[dD]ata$",
		// 				},
		// 			},
		// 		},
		// 	},
		// },
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redact-generate-kubeconfig",
			},
			Spec: auditlogv1.AuditLogPolicySpec{
				Filters: []auditlogv1.Filter{
					{
						Action:     auditlogv1.FilterActionAllow,
						RequestURI: ".*action=generateKubeconfig.*",
					},
				},
				AdditionalRedactions: []auditlogv1.Redaction{
					{
						Keys: []string{
							"config",
						},
					},
				},
			},
		},
	}
}
