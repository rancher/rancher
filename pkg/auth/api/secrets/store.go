package secrets

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
)

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
	fields, ok := TypeToFields[kind]
	subFields, subOk := SubTypeToFields[kind]
	if !ok && !subOk {
		return s.Store.Update(apiContext, schema, data, id)
	}

	var err error
	for _, field := range fields {
		if val, ok := data[field]; ok {
			data[field], err = s.CreateOrUpdateSecrets(convert.ToString(val), field, kind)
			if err != nil {
				return nil, err
			}
		}
	}

	// subfields for embedded configs, see saml group search using openldap
	for subField, subFieldList := range subFields {
		if subData, ok := data[subField]; ok {
			subData, casteOk := subData.(map[string]interface{})
			if !casteOk {
				continue
			}
			for _, field := range subFieldList {
				if val, ok := subData[field]; ok {
					subData[field], err = s.CreateOrUpdateSecrets(convert.ToString(val), field, kind)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	return s.Store.Update(apiContext, schema, data, id)
}

func (s *Store) CreateOrUpdateSecrets(value, field, kind string) (string, error) {
	if err := common.CreateOrUpdateSecrets(s.Secrets, value, strings.ToLower(field), strings.ToLower(kind)); err != nil {
		return "", fmt.Errorf("error creating secret for %s:%s", kind, field)
	}
	return fmt.Sprintf("%s:%s-%s", namespace.GlobalNamespace, strings.ToLower(kind), strings.ToLower(field)), nil
}
