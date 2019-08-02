package storageclass

import (
	"strings"

	"github.com/rancher/norman/types"
)

const (
	StorageAccountType = "storageaccounttype"
	SkuName            = "skuName"
	storageKind        = "kind"
	defaultStorageType = "Standard_LRS"
	parameters         = "parameters"
	provisioner        = "provisioner"
	AzureDisk          = "kubernetes.io/azure-disk"
)

func Wrap(store types.Store) types.Store {
	return &storageClassStore{
		store,
	}
}

type storageClassStore struct {
	types.Store
}

func (s *storageClassStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if data[provisioner] == AzureDisk {
		params, ok := data[parameters].(map[string]interface{})
		if !ok {
			params = make(map[string]interface{})
			data[parameters] = params
		}

		kind, _ := params[storageKind].(string)
		kind = strings.ToLower(kind)
		if kind == "shared" || kind == "" {
			saType, _ := params[StorageAccountType].(string)
			skuName, _ := params[SkuName].(string)

			if saType == "" && skuName == "" {
				params[StorageAccountType] = defaultStorageType
			}
		}
	}

	return s.Store.Create(apiContext, schema, data)
}
