package auth

import (
	"fmt"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/namespace"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/client/management/v3"
	"strings"
)

var TypeToField = map[string]string{
	client.GithubConfigType:          "clientSecret",
	client.ActiveDirectoryConfigType: "serviceAccountPassword",
	client.AzureADConfigType:         "applicationSecret",
	client.OpenLdapConfigType:        "serviceAccountPassword",
	client.FreeIpaConfigType:         "serviceAccountPassword",
	client.PingConfigType:            "spKey",
	client.ADFSConfigType:            "spKey",
	client.KeyCloakConfigType:        "spKey",
	client.OKTAConfigType:            "spKey",
}

func Wrap(store types.Store, secrets corev1.SecretInterface) types.Store {
	return &Store{
		Store:   store,
		Secrets: secrets,
	}
}

type Store struct {
	types.Store
	Secrets corev1.SecretInterface
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	for kind, field := range TypeToField {
		if val, ok := data[field]; ok {
			val := convert.ToString(val)
			if err := common.CreateOrUpdateSecrets(s.Secrets, val, strings.ToLower(field), strings.ToLower(kind)); err != nil {
				return nil, fmt.Errorf("error creating secret for %s:%s", kind, field)
			}
			data[field] = fmt.Sprintf("%s:%s-%s", namespace.GlobalNamespace, strings.ToLower(kind), strings.ToLower(field))
			break
		}
	}
	return s.Store.Update(apiContext, schema, data, id)
}
