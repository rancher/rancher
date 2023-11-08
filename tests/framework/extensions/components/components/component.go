package components

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	log "github.com/sirupsen/logrus"
)

func GenericCreate(client *rancher.Client, obj interface{}, objType string) (result interface{}, err error) {
	log.Info("In Create")
	client.Steve.SteveType(objType).Create(obj)
	return obj, err
}
