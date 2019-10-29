package nodetemplate

import (
	"fmt"
	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/controllers/management/node"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/ref"
	corev1 "github.com/rancher/types/apis/core/v1"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

type nodeTemplateStore struct {
	types.Store
	NodePoolLister        v3.NodePoolLister
	CloudCredentialLister corev1.SecretLister
	NodeTemplateClient    v3.NodeTemplateInterface
}

func Wrap(store types.Store, npLister v3.NodePoolLister, secretLister corev1.SecretLister, ntClient v3.NodeTemplateInterface) types.Store {
	s := &nodeTemplateStore{
		Store:                 store,
		NodePoolLister:        npLister,
		CloudCredentialLister: secretLister,
		NodeTemplateClient:    ntClient,
	}
	return &transform.Store{
		Store:       s,
		Transformer: s.filterLegacyTemplates,
	}
}

func (s *nodeTemplateStore) filterLegacyTemplates(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
	ns, _ := data["namespaceId"].(string)
	name, _ := data["name"].(string)
	if ns != namespace.NodeTemplateGlobalNamespace {
		s.NodeTemplateClient.Controller().Enqueue(ns, name)
		return nil, nil
	}
	return data, nil
}

func (s *nodeTemplateStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
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

func (s *nodeTemplateStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if err := s.replaceCloudCredFields(apiContext, data); err != nil {
		return data, err
	}
	return s.Store.Create(apiContext, schema, data)
}

func (s *nodeTemplateStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := s.replaceCloudCredFields(apiContext, data); err != nil {
		return data, err
	}
	return s.Store.Update(apiContext, schema, data, id)
}

func (s *nodeTemplateStore) replaceCloudCredFields(apiContext *types.APIContext, data map[string]interface{}) error {
	credID := convert.ToString(values.GetValueN(data, "cloudCredentialId"))
	if credID == "" {
		return nil
	}
	var accessCred client.CloudCredential
	if err := access.ByID(apiContext, &managementschema.Version, client.CloudCredentialType, credID, &accessCred); err != nil {
		if apiError, ok := err.(*httperror.APIError); ok {
			if apiError.Code.Status == httperror.PermissionDenied.Status || apiError.Code.Status == httperror.NotFound.Status {
				return httperror.NewAPIError(httperror.NotFound, fmt.Sprintf("cloud credential not found"))
			}
		}
		return httperror.WrapAPIError(err, httperror.ServerError, fmt.Sprintf("error accessing cloud credential"))
	}
	ns, name := ref.Parse(credID)
	if ns == "" || name == "" {
		return httperror.NewAPIError(httperror.InvalidReference, fmt.Sprintf("invalid cloud credential %s", credID))
	}
	cred, err := s.CloudCredentialLister.Get(namespace.GlobalNamespace, name)
	if err != nil {
		return httperror.WrapAPIError(err, httperror.ServerError, fmt.Sprintf("error getting cloud cred %s: %v", credID, err))
	}
	if len(cred.Data) == 0 {
		return httperror.WrapAPIError(err, httperror.MissingRequired, fmt.Sprintf("empty credID data %s", credID))
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
		return httperror.WrapAPIError(err, httperror.MissingRequired, fmt.Sprintf("empty configName for credID %s", configName))
	}
	toReplace := convert.ToMapInterface(values.GetValueN(data, configName))
	if len(toReplace) == 0 {
		return nil
	}
	driverNameSplit := strings.SplitN(configName, "Config", 2)
	if len(driverNameSplit) == 0 {
		return httperror.WrapAPIError(err, httperror.MissingRequired, fmt.Sprintf("empty driverName for credID %s", configName))
	}
	ignoreFields, ignore := node.IgnoreCredFieldForTemplate[driverNameSplit[0]]
	var fields []string
	for key := range cred.Data {
		splitKey := strings.SplitN(key, "-", 2)
		if len(splitKey) == 2 && splitKey[0] == credConfigName {
			if ignore && ignoreFields[splitKey[1]] {
				continue
			}
			delete(toReplace, splitKey[1])
			fields = append(fields, splitKey[1])
		}
	}
	logrus.Debugf("replaceCloudCredFields: %v for credID %s", fields, credID)
	return nil
}
