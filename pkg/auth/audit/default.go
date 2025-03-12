package audit

import (
	"fmt"
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

	sensitiveBodyFields = []string{"credentials", "applicationSecret", "oauthCredential", "serviceAccountCredential", "spKey", "spCert", "certificate", "privateKey"}
)

var (
	defaultRegex     = ".*([pP]assword|[Kk]ube[Cc]onfig|[Tt]oken).*"
	defaultRedactors []Redactor
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

	r, err := regexRedactor([]string{defaultRegex})
	if err != nil {
		panic(fmt.Sprintf("failed to create regex redactor: %v", err))
	}

	defaultRedactors = []Redactor{
		RedactFunc(redactSecret),
		r,
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
				Filters: []auditlogv1.Filter{},
				AdditionalRedactions: []auditlogv1.Redaction{
					{
						Headers: []string{
							strings.Join(sensitiveHeaders, "|"),
						},
						Paths: []string{
							fmt.Sprintf("$..[%s]", strings.Join(sensitiveBodyFields, ",")),
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redact-generate-kubeconfig",
			},
			Spec: auditlogv1.AuditLogPolicySpec{
				Filters: []auditlogv1.Filter{},
				AdditionalRedactions: []auditlogv1.Redaction{
					{
						Paths: []string{
							"$..config",
						},
					},
				},
			},
		},
	}
}
