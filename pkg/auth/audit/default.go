package audit

import (
	"fmt"
	"strings"
	"sync"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/data/management"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	sensitiveHeaders = []string{
		// Request Headers
		"Cookie", "Authorization", "X-Api-Tunnel-Params", "X-Api-Tunnel-Token", "X-Api-Auth-Header", "X-Amz-Security-Token",

		// Response Headers
		"Cookie", "Set-Cookie", "X-Api-Set-Cookie-Header",
	}

	sensitiveBodyFields = []string{
		"credentials", "applicationSecret", "oauthCredential", "serviceAccountCredential", "spKey", "spCert", "certificate", "privateKey", "secretsEncryptionConfig", "manifestUrl",
		"insecureWindowsNodeCommand", "insecureNodeCommand", "insecureCommand", "command", "nodeCommand", "windowsNodeCommand", "clientRandom",
	}
)

var (
	defaultRegex = ".*([pP]assword|[Kk]ube[Cc]onfig|[Tt]oken).*"

	defaultMu        sync.Mutex
	defaultRedactors []Redactor
)

func init() {
	for _, fields := range management.DriverData {
		for _, item := range fields.PrivateCredentialFields {
			sensitiveBodyFields = append(sensitiveBodyFields, item)
		}

		for _, item := range fields.PasswordFields {
			sensitiveBodyFields = append(sensitiveBodyFields, item)
		}
	}

	for _, fields := range management.KEv2OperatorsCredentialFields {
		for fieldName, field := range fields {
			if field.Type == "password" {
				sensitiveBodyFields = append(sensitiveBodyFields, fieldName)
			}
		}
	}

	r, err := regexRedactor([]string{defaultRegex})
	if err != nil {
		panic(fmt.Sprintf("failed to create regex redactor: %v", err))
	}

	defaultMu.Lock()
	defaultRedactors = []Redactor{
		RedactFunc(redactSecret),
		RedactFunc(redactConfigMap),
		RedactFunc(redactImportUrl),
		r,
	}
	defaultMu.Unlock()
}

// DefaultPolicy is the policy deployed by default by rancher and cannot currently be disabled once it is installed.
func DefaultPolicies() []auditlogv1.AuditPolicy {
	return []auditlogv1.AuditPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redact-generic-info",
			},
			Spec: auditlogv1.AuditPolicySpec{
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
			Spec: auditlogv1.AuditPolicySpec{
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "redact-last-applied-configuration",
			},
			Spec: auditlogv1.AuditPolicySpec{
				Filters: []auditlogv1.Filter{
					{
						Action:     auditlogv1.FilterActionAllow,
						RequestURI: ".*secrets.*",
					},
				},
				AdditionalRedactions: []auditlogv1.Redaction{
					{
						Paths: []string{
							"$..metadata.annotations['kubectl.kubernetes.io/last-applied-configuration']",
						},
					},
				},
			},
		},
	}
}
