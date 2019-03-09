package nodetemplate

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/configfield"
)

const (
	Amazonec2driver    = "amazonec2"
	Azuredriver        = "azure"
	Vmwaredriver       = "vmwarevsphere"
	DigitalOceandriver = "digitalocean"
)

var drivers = map[string]bool{
	Amazonec2driver:    true,
	Azuredriver:        true,
	DigitalOceandriver: true,
	Vmwaredriver:       true,
}

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	driver := configfield.GetDriver(data)
	if driver == "" {
		return httperror.NewAPIError(httperror.MissingRequired, "a Config field must be set")
	}
	credID := convert.ToString(values.GetValueN(data, "cloudCredentialId"))
	if credID == "" {
		if _, ok := drivers[driver]; ok {
			return httperror.NewAPIError(httperror.MissingRequired, "Cloud Credential must be set")
		}
	}
	if data != nil {
		data["driver"] = driver
	}
	return nil
}
