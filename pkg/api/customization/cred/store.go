package cred

import (
	"encoding/base64"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/api/customization/globalresource"
	"github.com/rancher/types/apis/core/v1"
	"strings"
)

func Wrap(store types.Store, ns v1.NamespaceInterface) types.Store {
	transformStore := &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			if configExists(data) {
				data["type"] = "cloudCredential"
				if err := decodeNonPasswordFields(data); err != nil {
					return nil, err
				}
				return data, nil
			}
			return nil, nil
		},
	}
	return &globalresource.GlobalNamespaceStore{
		Store:              transformStore,
		NamespaceInterface: ns,
	}
}

func configExists(data map[string]interface{}) bool {
	for key, val := range data {
		if strings.HasSuffix(key, "Config") {
			if convert.ToString(val) != "" {
				return true
			}
		}
	}
	return false
}

func decodeNonPasswordFields(data map[string]interface{}) error {
	for key, val := range data {
		if strings.HasSuffix(key, "Config") {
			ans := convert.ToMapInterface(val)
			for field, value := range ans {
				decoded, err := base64.StdEncoding.DecodeString(convert.ToString(value))
				if err != nil {
					return err
				}
				ans[field] = string(decoded)
			}
		}
	}
	return nil
}

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	if !configExists(data) {
		return httperror.NewAPIError(httperror.MissingRequired, "a Config field must be set")
	}

	return nil
}
