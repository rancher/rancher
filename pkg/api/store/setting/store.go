package setting

import (
	"os"

	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/settings"
)

func New(store types.Store) types.Store {
	return &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			v, ok := data["value"]
			value := os.Getenv(settings.GetENVKey(apiContext.ID))
			switch {
			case value != "":
				data["value"] = value
				data["customized"] = false
				data["source"] = "env"

			case !ok || v == "":
				data["value"] = data["default"]
				data["customized"] = false
				data["source"] = "default"
			default:
				data["customized"] = true
				data["source"] = "db"
			}
			return data, nil
		},
	}
}
