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
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtclient "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/resourcequota"
	mgmtschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const roleTemplatesRequired = "authz.management.cattle.io/creator-role-bindings"
const quotaField = "resourceQuota"
const namespaceQuotaField = "namespaceDefaultResourceQuota"

type projectStore struct {
	types.Store
	projectLister      v3.ProjectLister
	roleTemplateLister v3.RoleTemplateLister
	scaledContext      *config.ScaledContext
	clusterLister      v3.ClusterLister
}

func SetProjectStore(schema *types.Schema, mgmt *config.ScaledContext) {
	store := &projectStore{
		Store:              schema.Store,
		projectLister:      mgmt.Management.Projects("").Controller().Lister(),
		roleTemplateLister: mgmt.Management.RoleTemplates("").Controller().Lister(),
		scaledContext:      mgmt,
		clusterLister:      mgmt.Management.Clusters("").Controller().Lister(),
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
	} else if !quotaOk {
		return nil
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
	return s.isQuotaFit(apiContext, nsQuotaLimit, projectQuotaLimit, id)
}

func (s *projectStore) isQuotaFit(apiContext *types.APIContext, nsQuotaLimit *v32.ResourceQuotaLimit,
	projectQuotaLimit *v32.ResourceQuotaLimit, id string) error {
	// check that namespace default quota is within project quota
	isFit, msg, err := resourcequota.IsQuotaFit(nsQuotaLimit, []*v32.ResourceQuotaLimit{}, projectQuotaLimit)
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

	// check if fields were added or removed
	// and update project's namespaces accordingly
	defaultQuotaLimitMap, err := convert.EncodeToMap(nsQuotaLimit)
	if err != nil {
		return err
	}

	usedQuotaLimitMap := map[string]interface{}{}
	if project.ResourceQuota != nil && project.ResourceQuota.UsedLimit != nil {
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
	isFit, msg, err = resourcequota.IsQuotaFit(usedQuotaLimit, []*v32.ResourceQuotaLimit{}, projectQuotaLimit)
	if err != nil {
		return err
	}
	if !isFit {
		return httperror.NewFieldAPIError(httperror.MaxLimitExceeded, quotaField, fmt.Sprintf("is below the used limit on fields: %s",
			msg))
	}

	if len(limitToAdd) == 0 && len(limitToRemove) == 0 {
		return nil
	}

	// check if default quota is enough to set on namespaces
	toAppend := &mgmtclient.ResourceQuotaLimit{}
	if err := mapstructure.Decode(limitToAdd, toAppend); err != nil {
		return err
	}
	converted, err := limitToLimit(toAppend)
	if err != nil {
		return err
	}
	mu := resourcequota.GetProjectLock(id)
	mu.Lock()
	defer mu.Unlock()

	namespacesCount, err := s.getNamespacesCount(apiContext, project)
	if err != nil {
		return err
	}
	var nsLimits []*v32.ResourceQuotaLimit
	for i := 0; i < namespacesCount; i++ {
		nsLimits = append(nsLimits, converted)
	}

	isFit, msg, err = resourcequota.IsQuotaFit(&v32.ResourceQuotaLimit{}, nsLimits, projectQuotaLimit)
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

func (s *projectStore) getNamespacesCount(apiContext *types.APIContext, project mgmtclient.Project) (int, error) {
	cluster, err := s.clusterLister.Get("", project.ClusterID)
	if err != nil {
		return 0, err
	}

	kubeConfig, err := clustermanager.ToRESTConfig(cluster, s.scaledContext)
	if kubeConfig == nil || err != nil {
		return 0, err
	}

	clusterContext, err := config.NewUserContext(s.scaledContext, *kubeConfig, cluster.Name)
	if err != nil {
		return 0, err
	}

	namespaces, err := clusterContext.Core.Namespaces("").List(metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	count := 0
	for _, n := range namespaces.Items {
		if n.Annotations == nil {
			continue
		}
		if n.Annotations["field.cattle.io/projectId"] == project.ID {
			count++
		}
	}

	return count, nil
}

func limitToLimit(from *mgmtclient.ResourceQuotaLimit) (*v32.ResourceQuotaLimit, error) {
	var to v32.ResourceQuotaLimit
	err := convert.ToObj(from, &to)
	return &to, err
}
