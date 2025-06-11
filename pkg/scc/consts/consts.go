package consts

import "fmt"

const (
	DefaultSCCNamespace                  = "cattle-scc-system"
	SCCSystemCredentialsSecretNamePrefix = "scc-system-credentials-"
)

func SCCCredentialsSecretName(namePartIn string) string {
	return fmt.Sprintf("%s%s", SCCSystemCredentialsSecretNamePrefix, namePartIn)
}
