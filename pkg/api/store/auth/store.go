package auth

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/namespace"
	corev1 "github.com/rancher/types/apis/core/v1"
	client "github.com/rancher/types/client/management/v3"
)

var TypeToFields = map[string][]string{
	client.GithubConfigType:          {client.GithubConfigFieldClientSecret},
	client.ActiveDirectoryConfigType: {client.ActiveDirectoryConfigFieldServiceAccountPassword},
	client.AzureADConfigType:         {client.AzureADConfigFieldApplicationSecret},
	client.OpenLdapConfigType:        {client.LdapConfigFieldServiceAccountPassword},
	client.FreeIpaConfigType:         {client.LdapConfigFieldServiceAccountPassword},
	client.PingConfigType:            {client.PingConfigFieldSpKey},
	client.ADFSConfigType:            {client.ADFSConfigFieldSpKey},
	client.KeyCloakConfigType:        {client.KeyCloakConfigFieldSpKey},
	client.OKTAConfigType:            {client.OKTAConfigFieldSpKey},
	client.ShibbolethConfigType:      {client.ShibbolethConfigFieldSpKey, client.ShibbolethConfigFieldServiceAccountPassword},
	client.GoogleOauthConfigType:     {client.GoogleOauthConfigFieldOauthCredential, client.GoogleOauthConfigFieldServiceAccountCredential},
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
	authType, found := values.GetValue(data, "type")
	if !found {
		return nil, fmt.Errorf("invalid data for auth store update")
	}

	kind := convert.ToString(authType)
	fields := TypeToFields[kind]
	for _, field := range fields {
		if val, ok := data[field]; ok {
			val := convert.ToString(val)
			if err := common.CreateOrUpdateSecrets(s.Secrets, val, strings.ToLower(field), strings.ToLower(kind)); err != nil {
				return nil, fmt.Errorf("error creating secret for %s:%s", kind, field)
			}
			data[field] = fmt.Sprintf("%s:%s-%s", namespace.GlobalNamespace, strings.ToLower(kind), strings.ToLower(field))
		}
	}
	return s.Store.Update(apiContext, schema, data, id)
}
