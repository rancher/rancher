package nodetemplate

import (
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	driver := GetDriver(data)
	if driver == "" {
		return httperror.NewAPIError(httperror.MissingRequired, "a Config field must be set")
	}
	if data != nil {
		data["driver"] = driver
	}
	return nil
}

func GetDriver(obj interface{}) string {
	data, _ := convert.EncodeToMap(obj)
	driver := ""

	for k, v := range data {
		if !strings.HasSuffix(k, "Config") || convert.IsAPIObjectEmpty(v) {
			continue
		}

		driver = strings.TrimSuffix(k, "Config")
		break
	}

	return driver
}
