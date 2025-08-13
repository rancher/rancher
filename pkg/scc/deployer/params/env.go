package params

import (
	"os"
	"strings"
)

const DisableBuiltinSCCOperator = "CATTLE_DISABLE_BUILTIN_SCC_OPERATOR"

func GetBuiltinDisabledEnv() bool {
	envVal, hasEnv := os.LookupEnv(DisableBuiltinSCCOperator)
	if !hasEnv {
		return false
	}

	lowerVal := strings.ToLower(envVal)
	switch lowerVal {
	case "1", "true", "yes", "on", "once":
		return true
	}

	return false
}
