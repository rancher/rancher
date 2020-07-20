package clustertemplate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/rancher/norman/httperror"
)

const (
	RKEConfigK8sVersion = "rancherKubernetesEngineConfig.kubernetesVersion"
)

func CheckKubernetesVersionFormat(k8sVersion string) (bool, error) {
	if !strings.Contains(k8sVersion, "-rancher") {
		errMsg := fmt.Sprintf("Requested kubernetesVersion %v is not of valid semver [major.minor.patch] format", k8sVersion)
		vparts := strings.Split(k8sVersion, ".")
		if len(vparts) != 3 {
			return false, httperror.NewAPIError(httperror.InvalidFormat, errMsg)
		}
		for _, part := range vparts[:2] {
			//part should be a numeric value
			if _, err := strconv.Atoi(part); err != nil {
				return false, httperror.NewAPIError(httperror.InvalidFormat, errMsg)
			}
		}
		_, err := semver.ParseRange("=" + k8sVersion)
		if err != nil {
			return false, httperror.NewAPIError(httperror.InvalidFormat, errMsg)
		}

		if vparts[2] == "x" {
			return true, nil
		}
	}
	return false, nil
}
