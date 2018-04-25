package drivers

import (
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	RefsBranchPrefix = "refs/heads/"
	RefsTagPrefix    = "refs/tags/"
)

func VerifyBranch(config *v3.SourceCodeConfig, branch string) bool {
	if config.BranchCondition == "all" {
		return true
	} else if config.BranchCondition == "except" {
		if config.Branch == branch {
			return false
		}
		return true
	} else if config.Branch == branch {
		return true

	}

	return false
}
