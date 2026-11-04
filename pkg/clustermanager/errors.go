package clustermanager

import (
	"errors"

	"github.com/rancher/norman/httperror"
)

// IsClusterUnavailableErr checks if a given error indicates that the requested cluster was not available
func IsClusterUnavailableErr(err error) bool {
	var apiError *httperror.APIError
	if errors.As(err, &apiError) {
		return apiError.Code == httperror.ClusterUnavailable
	}

	return false
}
