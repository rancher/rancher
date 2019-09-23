package cred

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/api/customization/namespacedresource"
	"github.com/rancher/rancher/pkg/namespace"
	v1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/labels"
)

func Wrap(store types.Store, ns v1.NamespaceInterface, nodeTemplateLister v3.NodeTemplateLister) types.Store {
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

	newStore := &Store{
		transformStore,
		nodeTemplateLister,
	}

	return namespacedresource.Wrap(newStore, ns, namespace.GlobalNamespace)
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

type Store struct {
	types.Store
	NodeTemplateLister v3.NodeTemplateLister
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	nodeTemplates, err := s.NodeTemplateLister.List("", labels.NewSelector())
	if err != nil {
		return nil, err
	}
	if len(nodeTemplates) > 0 {
		for _, template := range nodeTemplates {
			if template.Spec.CloudCredentialName != id {
				continue
			}
			return nil, httperror.NewAPIError(httperror.MethodNotAllowed, fmt.Sprintf("cloud credential is currently referenced by node template %s", template.Name))
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}
