package consts

import "fmt"

const (
	DefaultSCCNamespace = "cattle-scc-system"
)

const (
	ResourceSCCEntrypointSecretName      = "scc-registration"
	SCCSystemCredentialsSecretNamePrefix = "scc-system-credentials-"
	OfflineRequestSecretNamePrefix       = "offline-request-"
)

func SCCCredentialsSecretName(namePartIn string) string {
	return fmt.Sprintf("%s%s", SCCSystemCredentialsSecretNamePrefix, namePartIn)
}

func OfflineRequestSecretName(namePartIn string) string {
	return fmt.Sprintf("%s%s", OfflineRequestSecretNamePrefix, namePartIn)
}

const (
	ManagedBySecretBroker = "secret-broker"
)

const (
	FinalizerSccCredentials  = "scc.cattle.io/managed-credentials"
	FinalizerSccRegistration = "scc.cattle.io/managed-registration"
)

const (
	LabelObjectSalt       = "scc.cattle.io/instance-salt"
	LabelNameSuffix       = "scc.cattle.io/related-name-suffix"
	LabelSccHash          = "scc.cattle.io/scc-hash"
	LabelSccLastProcessed = "scc.cattle.io/last-processsed"
	LabelSccManagedBy     = "scc.cattle.io/managed-by"
	LabelSccSecretRole    = "scc.cattle.io/secret-role"
)

const (
	SecretKeyRegistrationCode  = "regCode"
	SecretKeyOfflineRegRequest = "request"
	SecretKeyOfflineRegCert    = "certificate"
)

type SecretRole string

const (
	SCCCredentialsRole SecretRole = "scc-credentials"
	OfflineRequestRole SecretRole = "offline-request"
	OfflineCertificate SecretRole = "offline-certificate"
)
