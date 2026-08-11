package operations

import "strings"

// componentOuterArgument returns the wrapper flag for a control-plane component.
func componentOuterArgument(component string) string {
	switch component {
	case KubeControllerManagerProbeName:
		return "--" + KubeControllerManagerArg
	case KubeSchedulerProbeName:
		return "--" + KubeSchedulerArg
	default:
		return ""
	}
}

// componentTLSSettingsFromOuterArgs extracts component TLS settings from repeated
// wrapper arguments (for example: --kube-scheduler-arg tls-cert-file=...).
func componentTLSSettingsFromOuterArgs(args []string, component string) ComponentTLSSettings {
	outer := componentOuterArgument(component)
	if outer == "" {
		return ComponentTLSSettings{}
	}
	return componentTLSSettingsFromKeyValueArgs(newArguments(args).Values(outer))
}

// componentTLSSettingsFromConfigArg extracts component TLS settings from a
// flat key=value list represented as string, []string, or []any values.
func componentTLSSettingsFromConfigArg(arg any) ComponentTLSSettings {
	switch v := arg.(type) {
	case []any:
		return componentTLSSettingsFromKeyValueArgs(convertInterfaceSliceToStringSlice(v))
	case []string:
		return componentTLSSettingsFromKeyValueArgs(v)
	case string:
		return componentTLSSettingsFromKeyValueArgs([]string{v})
	default:
		return ComponentTLSSettings{}
	}
}

// componentTLSSettingsFromKeyValueArgs extracts secure-port and explicit
// certificate/key paths from key=value arguments.
func componentTLSSettingsFromKeyValueArgs(args []string) ComponentTLSSettings {
	var settings ComponentTLSSettings
	for _, argument := range args {
		key, value, ok := strings.Cut(argument, "=")
		if !ok {
			continue
		}
		switch key {
		case SecurePortArgument:
			settings.SecurePort = value
		case TLSCertFileArgument:
			settings.TLSCertFile = value
		case TLSPrivateKeyFile:
			settings.TLSPrivateKeyFile = value
		}
	}
	return settings
}

// secureProbeArguments returns non-sensitive probe arguments. The private-key
// path is never included; an explicit certificate path is included whenever set.
func secureProbeArguments(settings ComponentTLSSettings) []string {
	args := make([]string, 0, 2)
	if settings.SecurePort != "" {
		args = append(args, SecurePortArgument+"="+settings.SecurePort)
	}
	if settings.TLSCertFile != "" {
		args = append(args, TLSCertFileArgument+"="+settings.TLSCertFile)
	}
	return args
}
