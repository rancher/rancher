package workload

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
)

// Validator validates deprecated fields `environment` and `environmentFrom` are not being used.
// These fields were deprecated in favor of the k8s native fields `env` and `envFrom`. See https://github.com/rancher/rancher/issues/16148
func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if containers, _ := values.GetSlice(data, "containers"); len(containers) > 0 {
		for _, c := range containers {
			if !convert.IsAPIObjectEmpty(values.GetValueN(c, "environment")) {
				return httperror.NewAPIError(httperror.InvalidBodyContent, "field `environment` is deprecated, please use Kubernetes native field `env` for the container's environment variables")
			}
			if !convert.IsAPIObjectEmpty(values.GetValueN(c, "environmentFrom")) {
				return httperror.NewAPIError(httperror.InvalidBodyContent, "field `environmentFrom` is deprecated, please use Kubernetes native fields `env` and `valueFrom` for the container's environment variables")
			}
		}
	}
	return nil
}
