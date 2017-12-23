package schema

import (
	"encoding/base64"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type RegistryCredentialMapper struct {
}

func (e RegistryCredentialMapper) FromInternal(data map[string]interface{}) {
}

func (e RegistryCredentialMapper) ToInternal(data map[string]interface{}) {
	if data == nil {
		return
	}

	auth := convert.ToString(data["auth"])
	username := convert.ToString(data["username"])
	password := convert.ToString(data["password"])

	if auth == "" && username != "" && password != "" {
		data["auth"] = base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	}
}

func (e RegistryCredentialMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
