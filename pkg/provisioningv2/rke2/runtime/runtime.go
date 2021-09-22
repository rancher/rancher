package runtime

import (
	"strings"
)

var (
	RuntimeK3S  = "k3s"
	RuntimeRKE2 = "rke2"
)

func GetRuntimeCommand(kubernetesVersion string) string {
	return strings.ToLower(GetRuntime(kubernetesVersion))
}

func GetRuntimeServerUnit(kubernetesVersion string) string {
	if GetRuntime(kubernetesVersion) == RuntimeK3S {
		return RuntimeK3S
	}
	return RuntimeRKE2 + "-server"
}

func GetRuntimeEnv(kubernetesVersion string) string {
	return strings.ToUpper(GetRuntime(kubernetesVersion))
}

func GetRuntime(kubernetesVersion string) string {
	if strings.Contains(kubernetesVersion, RuntimeK3S) {
		return RuntimeK3S
	}
	return RuntimeRKE2
}

func GetRuntimeSupervisorPort(kubernetesVersion string) int {
	if GetRuntime(kubernetesVersion) == RuntimeRKE2 {
		return 9345
	}
	return 6443
}
