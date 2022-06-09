package catalog

import (
	"context"
	"fmt"
	"strconv"

	"github.com/rancher/norman/store/proxy"
	"github.com/rancher/norman/store/transform"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/pkg/catalog/manager"
	catUtil "github.com/rancher/rancher/pkg/catalog/utils"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	hcommon "github.com/rancher/rancher/pkg/controllers/managementuserlegacy/helm/common"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
)

type templateStore struct {
	types.Store
	CatalogTemplateVersionLister v3.CatalogTemplateVersionLister
	CatalogManager               manager.CatalogManager
}

func GetTemplateStore(ctx context.Context, managementContext *config.ScaledContext) types.Store {
	ts := templateStore{
		CatalogTemplateVersionLister: managementContext.Management.CatalogTemplateVersions("").Controller().Lister(),
		CatalogManager:               managementContext.CatalogManager,
	}

	s := &transform.Store{
		Store: proxy.NewProxyStore(ctx, managementContext.ClientGetter,
			config.ManagementStorageContext,
			scheme.Scheme,
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
	versionMaps, ok := resource[client.CatalogTemplateFieldVersions].([]interface{})
	if ok {
		for _, versionData := range versionMaps {
			revision := ""
			if v, ok := versionData.(map[string]interface{})["revision"].(int64); ok {
				revision = strconv.FormatInt(v, 10)
			}
			versionString, ok := versionData.(map[string]interface{})["version"].(string)
			if !ok {
				logrus.Trace("[templateStore] failed type assertion for field \"version\" for CatalogTemplateFieldVersion")
				continue
			}
			versionID := fmt.Sprintf("%v-%v", resource["id"], versionString)
			if revision != "" {
				versionID = fmt.Sprintf("%v-%v", resource["id"], revision)
			}
			versionLink := apiContext.URLBuilder.ResourceLinkByID(schema, versionID)
			currentVersion := apiContext.Query.Get("currentVersion")
			if currentVersion != "" && currentVersion == versionString {
				r[versionString] = versionLink
				continue
			}
			if t.isTemplateVersionCompatible(apiContext.Query.Get("clusterName"), versionData.(map[string]interface{})["externalId"].(string)) {
				r[versionString] = versionLink
			}
		}
	}
	return r
}

// templateVersionForRancherVersion indicates if a templateVersion works with the rancher server version
// In the error case it will always return true - if a template is actually invalid for that rancher version
// API validation will handle the rejection
func (t *templateStore) isTemplateVersionCompatible(clusterName, externalID string) bool {
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

	err = t.CatalogManager.ValidateRancherVersion(template, "")
	if err != nil {
		return false
	}

	if clusterName != "" {
		if err := t.CatalogManager.ValidateKubeVersion(template, clusterName); err != nil {
			return false
		}
	}

	return true
}
