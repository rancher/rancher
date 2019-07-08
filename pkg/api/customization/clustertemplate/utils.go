package clustertemplate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/rancher/norman/httperror"
)

func CheckKubernetesVersionFormat(k8sVersion string) error {
	if !strings.Contains(k8sVersion, "-rancher") {
		errMsg := fmt.Sprintf("Requested kubernetesVersion %v is not of valid semver [major.minor.patch] format", k8sVersion)
		vparts := strings.Split(k8sVersion, ".")
		if len(vparts) != 3 {
			return httperror.NewAPIError(httperror.InvalidFormat, errMsg)
		}
		for _, part := range vparts[:2] {
			//part should be a numeric value
			if _, err := strconv.Atoi(part); err != nil {
				return httperror.NewAPIError(httperror.InvalidFormat, errMsg)
			}
		}
		_, err := semver.ParseRange("=" + k8sVersion)
		if err != nil {
			return httperror.NewAPIError(httperror.InvalidFormat, errMsg)
		}
	}
	return nil
}
