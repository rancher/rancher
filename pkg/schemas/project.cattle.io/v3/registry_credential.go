package schema

import (
	"encoding/base64"
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
)

type RegistryCredentialMapper struct {
}

func (e RegistryCredentialMapper) FromInternal(data map[string]interface{}) {
}

func (e RegistryCredentialMapper) ToInternal(data map[string]interface{}) error {
	if data == nil {
		return nil
	}

	if data["kind"] != "dockerCredential" {
		return nil
	}

	addAuthInfo(data)

	return nil
}

func addAuthInfo(data map[string]interface{}) error {

	registryMap := convert.ToMapInterface(data["registries"])
	for _, regCred := range registryMap {
		regCredMap := convert.ToMapInterface(regCred)

		username := convert.ToString(regCredMap["username"])
		if username == "" {
			continue
		}
		password := convert.ToString(regCredMap["password"])
		if password == "" {
			continue
		}
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
		regCredMap["auth"] = auth
	}

	return nil
}
func (e RegistryCredentialMapper) ModifySchema(schema *types.Schema, schemas *types.Schemas) error {
	return nil
}
