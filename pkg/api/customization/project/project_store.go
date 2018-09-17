package project

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/resourcequota"
	clusterchema "github.com/rancher/types/apis/cluster.cattle.io/v3/schema"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	mgmtschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	clusterclient "github.com/rancher/types/client/cluster/v3"
	mgmtclient "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
	"k8s.io/apimachinery/pkg/labels"
)

const roleTemplatesRequired = "authz.management.cattle.io/creator-role-bindings"
const quotaField = "resourceQuota"
const namespaceQuotaField = "namespaceDefaultResourceQuota"

type projectStore struct {
	types.Store
	projectLister      v3.ProjectLister
	roleTemplateLister v3.RoleTemplateLister
}

func SetProjectStore(schema *types.Schema, mgmt *config.ScaledContext) {
	store := &projectStore{
		Store:              schema.Store,
		projectLister:      mgmt.Management.Projects("").Controller().Lister(),
		roleTemplateLister: mgmt.Management.RoleTemplates("").Controller().Lister(),
	}
	schema.Store = store
}

func (s *projectStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	annotation, err := s.createProjectAnnotation()
	if err != nil {
		return nil, err
	}

	if err := s.validateResourceQuota(apiContext, data, ""); err != nil {
		return nil, err
	}

	values.PutValue(data, annotation, "annotations", roleTemplatesRequired)

	return s.Store.Create(apiContext, schema, data)
}

func (s *projectStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := s.validateResourceQuota(apiContext, data, id); err != nil {
		return nil, err
	}

	return s.Store.Update(apiContext, schema, data, id)
}

func (s *projectStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	parts := strings.Split(id, ":")

	proj, err := s.projectLister.Get(parts[0], parts[len(parts)-1])
	if err != nil {
		return nil, err
	}
	if proj.Labels["authz.management.cattle.io/system-project"] == "true" {
		return nil, httperror.NewAPIError(httperror.MethodNotAllowed, "System Project cannot be deleted")
	}
	return s.Store.Delete(apiContext, schema, id)
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

func (s *projectStore) validateResourceQuota(apiContext *types.APIContext, data map[string]interface{}, id string) error {
	quotaO, quotaOk := data[quotaField]
	if quotaO == nil {
		quotaOk = false
	}
	nsQuotaO, namespaceQuotaOk := data[namespaceQuotaField]
	if nsQuotaO == nil {
		namespaceQuotaOk = false
	}
	if quotaOk != namespaceQuotaOk {
		if quotaOk {
			return httperror.NewFieldAPIError(httperror.MissingRequired, namespaceQuotaField, "")
		}
		return httperror.NewFieldAPIError(httperror.MissingRequired, quotaField, "")
	}

	var nsQuota mgmtclient.NamespaceResourceQuota
	if err := convert.ToObj(nsQuotaO, &nsQuota); err != nil {
		return err
	}
	var projectQuota mgmtclient.ProjectResourceQuota
	if err := convert.ToObj(quotaO, &projectQuota); err != nil {
		return err
	}

	projectQuotaLimit, err := limitToLimit(projectQuota.Limit)
	if err != nil {
		return err
	}
	nsQuotaLimit, err := limitToLimit(nsQuota.Limit)
	if err != nil {
		return err
	}

	// limits in namespace default quota should include all limits defined in the project quota
	projectQuotaLimitMap, err := convert.EncodeToMap(projectQuotaLimit)
	if err != nil {
		return err
	}

	nsQuotaLimitMap, err := convert.EncodeToMap(nsQuotaLimit)
	if err != nil {
		return err
	}
	if len(nsQuotaLimitMap) != len(projectQuotaLimitMap) {
		return httperror.NewFieldAPIError(httperror.MissingRequired, namespaceQuotaField, fmt.Sprintf("does not have all fields defined on a %s", quotaField))
	}

	for k := range projectQuotaLimitMap {
		if _, ok := nsQuotaLimitMap[k]; !ok {
			return httperror.NewFieldAPIError(httperror.MissingRequired, namespaceQuotaField, fmt.Sprintf("misses %s defined on a %s", k, quotaField))
		}
	}
	return isQuotaFit(apiContext, nsQuotaLimit, projectQuotaLimit, id)
}

func isQuotaFit(apiContext *types.APIContext, nsQuotaLimit *v3.ResourceQuotaLimit,
	projectQuotaLimit *v3.ResourceQuotaLimit, id string) error {
	// check that namespace default quota is within project quota
	isFit, msg, err := resourcequota.IsQuotaFit(nsQuotaLimit, []*v3.ResourceQuotaLimit{}, projectQuotaLimit)
	if err != nil {
		return err
	}
	if !isFit {
		return httperror.NewFieldAPIError(httperror.MaxLimitExceeded, namespaceQuotaField, fmt.Sprintf("exceeds %s on fields: %s",
			quotaField, msg))
	}

	if id == "" {
		return nil
	}

	var project mgmtclient.Project
	if err := access.ByID(apiContext, &mgmtschema.Version, mgmtclient.ProjectType, id, &project); err != nil {
		return err
	}

	if project.ResourceQuota == nil {
		return nil
	}

	// check if fields were added or removed
	// and update project's namespaces accordingly
	defaultQuotaLimitMap, err := convert.EncodeToMap(nsQuotaLimit)
	if err != nil {
		return err
	}

	usedQuotaLimitMap := map[string]interface{}{}
	if project.ResourceQuota.UsedLimit != nil {
		usedQuotaLimitMap, err = convert.EncodeToMap(project.ResourceQuota.UsedLimit)
		if err != nil {
			return err
		}
	}

	limitToAdd := map[string]interface{}{}
	limitToRemove := map[string]interface{}{}
	for key, value := range defaultQuotaLimitMap {
		if _, ok := usedQuotaLimitMap[key]; !ok {
			limitToAdd[key] = value
		}
	}

	for key, value := range usedQuotaLimitMap {
		if _, ok := defaultQuotaLimitMap[key]; !ok {
			limitToRemove[key] = value
		}
	}

	// check that used quota is not bigger than the project quota
	for key := range limitToRemove {
		delete(usedQuotaLimitMap, key)
	}

	var usedLimitToCheck mgmtclient.ResourceQuotaLimit
	err = convert.ToObj(usedQuotaLimitMap, &usedLimitToCheck)
	if err != nil {
		return err
	}

	usedQuotaLimit, err := limitToLimit(&usedLimitToCheck)
	if err != nil {
		return err
	}
	isFit, msg, err = resourcequota.IsQuotaFit(usedQuotaLimit, []*v3.ResourceQuotaLimit{}, projectQuotaLimit)
	if err != nil {
		return err
	}
	if !isFit {
		return httperror.NewFieldAPIError(httperror.MaxLimitExceeded, quotaField, fmt.Sprintf("exceeds used limit on fields: %s",
			msg))
	}

	if len(limitToAdd) == 0 && len(limitToRemove) == 0 {
		return nil
	}

	// check if default quota is enough to set on namespaces
	mu := resourcequota.GetProjectLock(id)
	mu.Lock()
	defer mu.Unlock()
	var namespaces []clusterclient.Namespace
	options := &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString("projectId", types.ModifierEQ, id),
		},
	}
	if err := access.List(apiContext, &clusterchema.Version, clusterclient.NamespaceType, options, &namespaces); err != nil {
		return err
	}

	var nsLimits []*v3.ResourceQuotaLimit
	toAppend := &mgmtclient.ResourceQuotaLimit{}
	if err := mapstructure.Decode(limitToAdd, toAppend); err != nil {
		return err
	}
	converted, err := limitToLimit(toAppend)
	if err != nil {
		return err
	}
	for i := 0; i < len(namespaces); i++ {
		nsLimits = append(nsLimits, converted)
	}

	isFit, msg, err = resourcequota.IsQuotaFit(&v3.ResourceQuotaLimit{}, nsLimits, projectQuotaLimit)
	if err != nil {
		return err
	}
	if !isFit {
		return httperror.NewFieldAPIError(httperror.MaxLimitExceeded, namespaceQuotaField,
			fmt.Sprintf("exceeds project limit on fields %s when applied to all namespaces in a project",
				msg))
	}

	return nil
}

func limitToLimit(from *mgmtclient.ResourceQuotaLimit) (*v3.ResourceQuotaLimit, error) {
	var to v3.ResourceQuotaLimit
	err := convert.ToObj(from, &to)
	return &to, err
}
