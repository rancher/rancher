package configmaps

import (
	"github.com/pkg/errors"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	cm "github.com/rancher/rancher/tests/framework/extensions/configmaps"
	"github.com/sirupsen/logrus"
)

const (
	APIVersion = "v1"
	Kind       = "ConfigMap"
)

func setPayload(apiObject *steveV1.SteveAPIObject, payload *cm.SteveConfigMap) error {
	payload.APIVersion = APIVersion
	payload.Kind = Kind
	payload.Name = apiObject.Name
	payload.Namespace = "default"
	payload.ResourceVersion = apiObject.ResourceVersion
	payload.UID = apiObject.UID
	payload.State = &steveV1.State{}
	payload.Relationships = &[]steveV1.Relationship{}
	payload.Fields = []any{"foo", "bar"}
	data, ok := apiObject.JSONResp["data"].(map[string]interface{})
	if !ok {
		logrus.Println("Type assertion failed")
		return errors.New("type assertion failed for apiObject.JSONResp[\"data\"]")
	}
	payload.Data = data
	return nil
}
