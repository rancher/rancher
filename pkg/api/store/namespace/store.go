package namespace

import (
	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	"github.com/rancher/rancher/pkg/resourcequota"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	mgmtschema "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3/schema"
	clusterclient "github.com/rancher/rancher/pkg/types/client/cluster/v3"
	mgmtclient "github.com/rancher/rancher/pkg/types/client/management/v3"
)

const quotaField = "resourceQuota"
const containerRecsourceLimitField = "containerDefaultResourceLimit"

func New(store types.Store) types.Store {
	t := &transform.Store{
		Store: store,
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			anns, _ := data["annotations"].(map[string]interface{})
			if anns["management.cattle.io/system-namespace"] == "true" {
				return nil, nil
			}
			return data, nil
		},
	}

	return &Store{
		Store: t,
	}
}

type Store struct {
	types.Store
}

func (p *Store) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := data["resourceQuota"]; ok {
		values.PutValue(data, "{\"conditions\": [{\"type\": \"InitialRolesPopulated\", \"status\": \"Unknown\", \"message\": \"Populating initial roles\"},{\"type\": \"ResourceQuotaValidated\", \"status\": \"Unknown\", \"message\": \"Validating resource quota\"}]}",
			"annotations", "cattle.io/status")
	} else {
		values.PutValue(data, "{\"conditions\": [{\"type\": \"InitialRolesPopulated\", \"status\": \"Unknown\", \"message\": \"Populating initial roles\"}]}",
			"annotations", "cattle.io/status")
	}

	if err := p.validateResourceQuota(apiContext, schema, data, "", false); err != nil {
		return nil, err
	}

	return p.Store.Create(apiContext, schema, data)
}

func (p *Store) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	if err := p.validateResourceQuota(apiContext, schema, data, id, true); err != nil {
		return nil, err
	}

	return p.Store.Update(apiContext, schema, data, id)
}

func (p *Store) validateResourceQuota(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string, update bool) error {
	quota := data[quotaField]
	projectID := convert.ToString(data["projectId"])
	if update {
		var ns clusterclient.Namespace
		if err := access.ByID(apiContext, &schema.Version, clusterclient.NamespaceType, id, &ns); err != nil {
			return err
		}
		projectID = ns.ProjectID
	}
	if projectID == "" {
		return nil
	}
	var project mgmtclient.Project
	if err := access.ByID(apiContext, &mgmtschema.Version, mgmtclient.ProjectType, projectID, &project); err != nil {
		return err
	}
	if project.ResourceQuota == nil {
		return nil
	}
	var nsQuota mgmtclient.NamespaceResourceQuota
	if quota == nil {
		if project.NamespaceDefaultResourceQuota == nil {
			return nil
		}
		nsQuota = *project.NamespaceDefaultResourceQuota
	} else {
		if err := convert.ToObj(quota, &nsQuota); err != nil {
			return err
		}
	}

	projectQuotaLimit, err := limitToLimit(project.ResourceQuota.Limit)
	if err != nil {
		return err
	}
	nsQuotaLimit, err := limitToLimit(nsQuota.Limit)
	if err != nil {
		return err
	}

	// limits in namespace should include all limits defined on a project
	projectQuotaLimitMap, err := convert.EncodeToMap(projectQuotaLimit)
	if err != nil {
		return err
	}

	nsQuotaLimitMap, err := convert.EncodeToMap(nsQuotaLimit)
	if err != nil {
		return err
	}
	if len(nsQuotaLimitMap) != len(projectQuotaLimitMap) {
		return httperror.NewFieldAPIError(httperror.MissingRequired, quotaField, "does not have all fields defined on a project quota")
	}

	for k := range projectQuotaLimitMap {
		if _, ok := nsQuotaLimitMap[k]; !ok {
			return httperror.NewFieldAPIError(httperror.MissingRequired, quotaField, fmt.Sprintf("misses %s defined on a project quota", k))
		}
	}

	// validate resource quota
	mu := resourcequota.GetProjectLock(projectID)
	mu.Lock()
	defer mu.Unlock()

	var nsLimits []*v3.ResourceQuotaLimit
	var namespaces []clusterclient.Namespace
	options := &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString("projectId", types.ModifierEQ, projectID),
		},
	}
	if err := access.List(apiContext, &schema.Version, clusterclient.NamespaceType, options, &namespaces); err != nil {
		return err
	}
	for _, n := range namespaces {
		if n.ID == id {
			continue
		}
		if n.ResourceQuota == nil {
			continue
		}
		nsLimit, err := limitToLimitCluster(n.ResourceQuota.Limit)
		if err != nil {
			return err
		}
		nsLimits = append(nsLimits, nsLimit)
	}

	// set default resource limit
	limit := data[containerRecsourceLimitField]
	if limit == nil {
		data[containerRecsourceLimitField] = project.ContainerDefaultResourceLimit
	}

	isFit, msg, err := resourcequota.IsQuotaFit(nsQuotaLimit, nsLimits, projectQuotaLimit)
	if err != nil || isFit {
		return err
	}

	return httperror.NewFieldAPIError(httperror.MaxLimitExceeded, quotaField, fmt.Sprintf("exceeds projectLimit on fields: %s", msg))
}

func limitToLimit(from *mgmtclient.ResourceQuotaLimit) (*v3.ResourceQuotaLimit, error) {
	var to v3.ResourceQuotaLimit
	err := convert.ToObj(from, &to)
	return &to, err
}

func limitToLimitCluster(from *clusterclient.ResourceQuotaLimit) (*v3.ResourceQuotaLimit, error) {
	var to v3.ResourceQuotaLimit
	err := convert.ToObj(from, &to)
	return &to, err
}
