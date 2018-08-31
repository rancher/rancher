package project

import (
	"encoding/json"

	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
)

const roleTemplatesRequired = "authz.management.cattle.io/creator-role-bindings"
const quotaField = "resourceQuota"
const namespaceQuotaField = "namespaceDefaultResourceQuota"

type projectStore struct {
	types.Store
	roleTemplateLister v3.RoleTemplateLister
}

func SetProjectStore(schema *types.Schema, mgmt *config.ScaledContext) {
	store := &projectStore{
		Store:              schema.Store,
		roleTemplateLister: mgmt.Management.RoleTemplates("").Controller().Lister(),
	}
	schema.Store = store
}

func (s *projectStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	annotation, err := s.createProjectAnnotation()
	if err != nil {
		return nil, err
	}

	if err := s.validateResourceQuota(apiContext, schema, data, ""); err != nil {
		return nil, err
	}

	values.PutValue(data, annotation, "annotations", roleTemplatesRequired)

	return s.Store.Create(apiContext, schema, data)
}

func (s *projectStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := s.validateResourceQuota(apiContext, schema, data, id); err != nil {
		return nil, err
	}
	if err := s.validateResourceQuota(apiContext, schema, data, ""); err != nil {
		return nil, err
	}

	return s.Store.Update(apiContext, schema, data, id)
}

func (s *projectStore) createProjectAnnotation() (string, error) {
	rt, err := s.roleTemplateLister.List("", labels.NewSelector())
	if err != nil {
		return "", err
	}

	annoMap := make(map[string][]string)

	for _, role := range rt {
		if role.ProjectCreatorDefault && !role.Locked {
			annoMap["required"] = append(annoMap["required"], role.Name)
		}
	}

	d, err := json.Marshal(annoMap)
	if err != nil {
		return "", err
	}

	return string(d), nil
}

func (s *projectStore) validateResourceQuota(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) error {
	_, quotaOk := data[quotaField]
	_, namespaceQuotaOk := data[namespaceQuotaField]
	if quotaOk != namespaceQuotaOk {
		return httperror.NewFieldAPIError(httperror.MissingRequired, fmt.Sprintf("Both %s and %s are required", quotaField, namespaceQuotaField), "")
	}
	logrus.Errorf("quota is %v", data[quotaField])
	return nil
}
