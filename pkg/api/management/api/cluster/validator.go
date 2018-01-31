package cluster

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

const (
	specField = "spec"
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	spec, ok := data[specField]
	if !ok {
		return nil
	}

	specData, _ := convert.EncodeToMap(spec)

	found := false
	for k, v := range specData {
		if strings.HasSuffix(k, "Config") && !convert.IsEmpty(v) {
			found = true
			break
		}
	}

	if found {
		return nil
	}
	return httperror.NewAPIError(httperror.MissingRequired, "a Config field is required")

}
