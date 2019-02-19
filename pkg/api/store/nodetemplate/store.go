package nodetemplate

import (
	"fmt"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

type Store struct {
	types.Store
	NodePoolLister        v3.NodePoolLister
	CloudCredentialLister corev1.SecretLister
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
	if err := s.replaceCloudCredFields(data); err != nil {
		return data, err
	}
	return s.Store.Create(apiContext, schema, data)
}

func (s *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := s.replaceCloudCredFields(data); err != nil {
		return data, err
	}
	return s.Store.Update(apiContext, schema, data, id)
}

func (s *Store) replaceCloudCredFields(data map[string]interface{}) error {
	credID := convert.ToString(values.GetValueN(data, "cloudCredentialId"))
	if credID == "" {
		return nil
	}
	ns, name := ref.Parse(credID)
	if ns == "" || name == "" {
		return fmt.Errorf("invalid credID %s", credID)
	}
	cred, err := s.CloudCredentialLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		return fmt.Errorf("error getting cloud cred %s: %v", credID, err)
	}
	if len(cred.Data) == 0 {
		return fmt.Errorf("empty credID data %s", credID)
	}
	configName, credConfigName := "", ""
	for key := range cred.Data {
		splitKey := strings.SplitN(key, "-", 2)
		if len(splitKey) == 2 && strings.HasSuffix(splitKey[0], "credentialConfig") {
			configName = strings.Replace(splitKey[0], "credential", "", 1)
			credConfigName = splitKey[0]
			break
		}
	}
	if configName == "" {
		return fmt.Errorf("empty configName for credID %s", configName)
	}
	toReplace := convert.ToMapInterface(values.GetValueN(data, configName))
	if len(toReplace) == 0 {
		return nil
	}
	var fields []string
	for key := range cred.Data {
		splitKey := strings.SplitN(key, "-", 2)
		if len(splitKey) == 2 && splitKey[0] == credConfigName {
			delete(toReplace, splitKey[1])
			fields = append(fields, splitKey[1])
		}
	}
	logrus.Debugf("replaceCloudCredFields: %v for credID %s", fields, credID)
	return nil
}
