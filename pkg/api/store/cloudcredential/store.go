package cloudcredential

import (
	"encoding/json"
	"github.com/rancher/norman/types"
	"github.com/sirupsen/logrus"
)

type Store struct {
	types.Store
}

func (t *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	logrus.Info("data is here")
	ans, _ := json.Marshal(data)
	logrus.Info("ans %s", string(ans))

	ctonn, _ := json.Marshal(apiContext)
	logrus.Infof("api %s", string(ctonn))

	return t.Store.Create(apiContext, schema, data)
}
