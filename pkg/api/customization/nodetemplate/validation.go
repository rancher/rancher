package nodetemplate

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/configfield"
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	driver := configfield.GetDriver(data)
	if driver == "" {
		return httperror.NewAPIError(httperror.MissingRequired, "a Config field must be set")
	}

	if data != nil {
		data["driver"] = driver
	}

	return nil
}
