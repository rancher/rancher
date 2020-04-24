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
}

type transformer struct {
	NodeTemplateClient v3.NodeTemplateInterface
}

type nodeTemplateIDs struct {
	fullOriginalID string
	originalID     string
	originalNs     string
	fullMigratedID string
	migratedID     string
	migratedNs     string
}

func Wrap(store types.Store, npLister v3.NodePoolLister, secretLister corev1.SecretLister, ntClient v3.NodeTemplateInterface) types.Store {
	t := &transformer{
		NodeTemplateClient: ntClient,
	}
	s := &transform.Store{
		Store:       store,
		Transformer: t.filterLegacyTemplates,
	}
	return &nodeTemplateStore{
		Store:                 s,
		NodePoolLister:        npLister,
		CloudCredentialLister: secretLister,
	}
}

func (t *transformer) filterLegacyTemplates(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
	ns, _ := data["namespaceId"].(string)
	id, _ := data["id"].(string)
	if ns != namespace.NodeTemplateGlobalNamespace {
		// nodetemplate may not have been migrated yet, enqueue
		t.NodeTemplateClient.Controller().Enqueue(ns, strings.TrimPrefix(id, ns+":"))
		return nil, nil
	}
	return data, nil
}

func (s *nodeTemplateStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	ids := getAllIDs(id)
	data, err := s.Store.ByID(apiContext, schema, ids.fullMigratedID)
	if err != nil {
		return nil, replaceIDInError(err, ids.migratedID, ids.originalID, ids.migratedNs, ids.originalNs)
	}

	data["namespaceId"] = ids.originalNs
	data["id"] = ids.fullOriginalID
	return data, nil
}

// getAllIDs returns the namespace and trimmed ID of the original ID
// and its corresponding migrated ID
func getAllIDs(id string) nodeTemplateIDs {
	originalNs, originalID := ref.Parse(id)
	fullMigratedID := getMigratedID(originalNs, originalID)
	migratedNs, migratedID := ref.Parse(fullMigratedID)
	return nodeTemplateIDs{
		fullOriginalID: id,
		originalID:     originalID,
		originalNs:     originalNs,
		fullMigratedID: fullMigratedID,
		migratedID:     migratedID,
		migratedNs:     migratedNs,
	}
}

func getMigratedID(ns, id string) string {
	if ns == namespace.NodeTemplateGlobalNamespace {
		return fmt.Sprintf("%s:%s", ns, id)
	}

	return fmt.Sprintf("%s:nt-%s-%s", namespace.NodeTemplateGlobalNamespace, ns, id)
}

func replaceIDInError(err error, id, newID, ns, newNs string) error {
	if apiError, ok := err.(*httperror.APIError); ok {
		msg := strings.Replace(apiError.Message, id, newID, -1)
		msg = strings.Replace(msg, ns, newNs, -1)
		err = httperror.NewAPIError(apiError.Code, msg)
	}
	return err
}

func (s *nodeTemplateStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	ids := getAllIDs(id)
	pools, err := s.NodePoolLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.Spec.NodeTemplateName == ids.fullMigratedID {
			return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "Template is in use by a node pool.")
		}
	}
	data, err := s.Store.Delete(apiContext, schema, ids.fullMigratedID)
	if err != nil {
		return nil, replaceIDInError(err, ids.migratedID, ids.originalID, ids.migratedNs, ids.originalNs)
	}

	if data != nil {
		data["namespaceId"] = ids.originalNs
		data["id"] = ids.fullOriginalID
	}

	return data, nil
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

	ids := getAllIDs(id)
	data, err := s.Store.Update(apiContext, schema, data, ids.fullMigratedID)
	if err != nil {
		return nil, replaceIDInError(err, ids.migratedID, ids.originalID, ids.migratedNs, ids.originalNs)
	}

	data["namespaceId"] = ids.originalNs
	data["id"] = ids.fullOriginalID
	return data, nil
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
