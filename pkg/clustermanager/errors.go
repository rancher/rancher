package clustermanager

import "github.com/rancher/norman/httperror"

// IsClusterUnavailableErr checks if a given error indicates that the requested cluster was not available
func IsClusterUnavailableErr(err error) bool {
	if apiError, ok := err.(*httperror.APIError); ok {
		return apiError.Code == httperror.ClusterUnavailable
	}
	return false
}
