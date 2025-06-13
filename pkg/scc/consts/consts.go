package consts

import "fmt"

const (
	DefaultSCCNamespace                  = "cattle-scc-system"
	SCCSystemCredentialsSecretNamePrefix = "scc-system-credentials-"
)

func SCCCredentialsSecretName(namePartIn string) string {
	return fmt.Sprintf("%s%s", SCCSystemCredentialsSecretNamePrefix, namePartIn)
}

const (
	ResourceSCCEntrypointSecretName = "scc-registration"
)

const (
	ManagedBySecretBroker = "secret-broker"
)

const (
	FinalizerSccCredentials  = "scc.cattle.io/managed-credentials"
	FinalizerSccRegistration = "scc.cattle.io/managed-registration"
)

const (
	LabelSccLastProcessed = "scc.cattle.io/last-processsed"
	LabelSccHash          = "scc.cattle.io/scc-hash"
	LabelSccManagedBy     = "scc.cattle.io/managed-by"
)
