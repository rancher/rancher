package nodeconfig

import (
	"github.com/rancher/rancher/pkg/encryptedstore"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
)

func NewStore(namespaceInterface v1.NamespaceInterface, secretsGetter v1.SecretsGetter) (*encryptedstore.GenericEncryptedStore, error) {
	return encryptedstore.NewGenericEncryptedStore("mc-", "", namespaceInterface, secretsGetter)
}
