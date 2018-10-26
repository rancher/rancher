package nodetemplate

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"

)

type Store struct {
	types.Store
	NodePoolLister v3.NodePoolLister
	Creds v3.CloudCredentialInterface
	Test types.Store
	TestSchema *types.Schema
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	pools, err := s.NodePoolLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Spec.NodeTemplateName == id {
			return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "Template is in use by a node pool.")
		}
	}
	return s.Store.Delete(apiContext, schema, id)
}

func (s *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	//logrus.Info("data is here")
	//ans, _ := json.Marshal(data)
	//logrus.Info("ans %s", string(ans))
	s.handleCredentials(apiContext, data)
	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) handleCredentials(apiContext *types.APIContext, data map[string]interface{}) {
	if _, ok := data["credentialId"]; ok {
		return
	}

	objData := map[string]interface{}{}
	driver := convert.ToString(data["driver"])

	read := driver + "Config"
	write := driver + "credentialConfig"

	value := convert.ToString(values.GetValueN(data, read, "accessToken"))

	objData[write] = map[string]string{"accessToken" : value}
	objData["name"] = convert.ToString(data["name"])+ "credential"

	apiContext.Type = client.CloudCredentialType

	if _, err := s.Test.Create(apiContext, s.TestSchema, objData); err != nil {
		logrus.Infof("etrrr %v",err)
	}
}