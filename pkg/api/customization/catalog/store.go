package catalog

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	catUtil "github.com/rancher/rancher/pkg/catalog/utils"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	hcommon "github.com/rancher/rancher/pkg/controllers/user/helm/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
)

type templateStore struct {
	types.Store
	CatalogTemplateVersionLister v3.CatalogTemplateVersionLister
}

func GetTemplateStore(ctx context.Context, managementContext *config.ScaledContext) types.Store {
	ts := templateStore{
		CatalogTemplateVersionLister: managementContext.Management.CatalogTemplateVersions("").Controller().Lister(),
	}

	s := &transform.Store{
		Store: proxy.NewProxyStore(ctx, managementContext.ClientGetter,
			config.ManagementStorageContext,
			[]string{"apis"},
			"management.cattle.io",
			"v3",
			"CatalogTemplate",
			"catalogtemplates"),
		Transformer: func(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, opt *types.QueryOptions) (map[string]interface{}, error) {
			data[client.CatalogTemplateFieldVersionLinks] = ts.extractVersionLinks(apiContext, data)
			return data, nil
		},
	}

	ts.Store = s

	return ts
}

func (t *templateStore) extractVersionLinks(apiContext *types.APIContext, resource map[string]interface{}) map[string]interface{} {
	schema := apiContext.Schemas.Schema(&managementschema.Version, client.TemplateVersionType)
	r := map[string]interface{}{}
	versionMap, ok := resource[client.CatalogTemplateFieldVersions].([]interface{})
	if ok {
		for _, version := range versionMap {
			revision := ""
			if v, ok := version.(map[string]interface{})["revision"].(int64); ok {
				revision = strconv.FormatInt(v, 10)
			}
			versionString := version.(map[string]interface{})["version"].(string)
			versionID := fmt.Sprintf("%v-%v", resource["id"], versionString)
			if revision != "" {
				versionID = fmt.Sprintf("%v-%v", resource["id"], revision)
			}
			if t.templateVersionForRancherVersion(apiContext, version.(map[string]interface{})["externalId"].(string)) {
				r[versionString] = apiContext.URLBuilder.ResourceLinkByID(schema, versionID)
			}
		}
	}
	return r
}

// templateVersionForRancherVersion indicates if a templateVersion works with the rancher server version
// In the error case it will always return true - if a template is actually invalid for that rancher version
// API validation will handle the rejection
func (t *templateStore) templateVersionForRancherVersion(apiContext *types.APIContext, externalID string) bool {
	rancherVersion := settings.ServerVersion.Get()

	if !catUtil.ReleaseServerVersion(rancherVersion) {
		return true
	}

	templateVersionID, namespace, err := hcommon.ParseExternalID(externalID)
	if err != nil {
		return true
	}

	template, err := t.CatalogTemplateVersionLister.Get(namespace, templateVersionID)
	if err != nil {
		return true
	}

	err = catUtil.ValidateRancherVersion(template)
	if err != nil {
		return false
	}
	return true
}
