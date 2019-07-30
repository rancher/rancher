package catalog

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	catUtil "github.com/rancher/rancher/pkg/catalog/utils"
	hcommon "github.com/rancher/rancher/pkg/controllers/user/helm/common"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/types/client/management/v3"
)

type TemplateWrapper struct {
	CatalogLister                v3.CatalogLister
	ClusterCatalogLister         v3.ClusterCatalogLister
	ProjectCatalogLister         v3.ProjectCatalogLister
	CatalogTemplateVersionLister v3.CatalogTemplateVersionLister
}

func (t TemplateWrapper) TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	var prjCatalogName, clusterCatalogName string
	// version links
	resource.Values["versionLinks"] = t.extractVersionLinks(apiContext, resource)

	//icon
	ic, ok := resource.Values["icon"]
	if ok {
		if strings.HasPrefix(ic.(string), "file:") {
			delete(resource.Values, "icon")
			resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)

		} else {
			delete(resource.Values, "icon")
			resource.Links["icon"] = ic.(string)
		}
	} else {
		delete(resource.Values, "icon")
		resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)
	}

	val := resource.Values
	if val[client.CatalogTemplateFieldCatalogID] != nil {
		//catalog link
		catalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.CatalogType)
		catalogName := strings.Split(resource.ID, "-")[0]
		resource.Links["catalog"] = apiContext.URLBuilder.ResourceLinkByID(catalogSchema, catalogName)
	}

	if val[client.CatalogTemplateFieldProjectCatalogID] != nil {
		prjCatID, ok := val[client.CatalogTemplateFieldProjectCatalogID].(string)
		if ok {
			prjCatalogName = prjCatID
		}
		//project catalog link
		prjCatalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.ProjectCatalogType)
		resource.Links["projectCatalog"] = apiContext.URLBuilder.ResourceLinkByID(prjCatalogSchema, prjCatalogName)
	}

	if val[client.CatalogTemplateFieldClusterCatalogID] != nil {
		clusterCatID, ok := val[client.CatalogTemplateFieldClusterCatalogID].(string)
		if ok {
			clusterCatalogName = clusterCatID
		}
		//cluster catalog link
		clCatalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.ClusterCatalogType)
		resource.Links["clusterCatalog"] = apiContext.URLBuilder.ResourceLinkByID(clCatalogSchema, clusterCatalogName)
	}

	// delete category
	delete(resource.Values, "category")

	// delete versions
	delete(resource.Values, "versions")
}

func (t TemplateWrapper) extractVersionLinks(apiContext *types.APIContext, resource *types.RawResource) map[string]string {
	schema := apiContext.Schemas.Schema(&managementschema.Version, client.TemplateVersionType)
	r := map[string]string{}
	versionMap, ok := resource.Values["versions"].([]interface{})
	if ok {
		for _, version := range versionMap {
			revision := ""
			if v, ok := version.(map[string]interface{})["revision"].(int64); ok {
				revision = strconv.FormatInt(v, 10)
			}
			versionString := version.(map[string]interface{})["version"].(string)
			versionID := fmt.Sprintf("%v-%v", resource.ID, versionString)
			if revision != "" {
				versionID = fmt.Sprintf("%v-%v", resource.ID, revision)
			}
			if t.templateVersionForRancherVersion(apiContext, version.(map[string]interface{})["externalId"].(string)) {
				r[versionString] = apiContext.URLBuilder.ResourceLinkByID(schema, versionID)
			}
		}
	}
	return r
}

func (t TemplateWrapper) TemplateIconHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "icon":
		template := &client.Template{}
		if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, template); err != nil {
			return err
		}
		if template.Icon == "" {
			http.Error(apiContext.Response, "", http.StatusNoContent)
			return nil
		}

		var (
			catalogType string
			catalogName string
			iconBytes   []byte
			err         error
		)

		if template.CatalogID != "" {
			catalogType = client.CatalogType
			catalogName = template.CatalogID
		} else if template.ClusterCatalogID != "" {
			catalogType = client.ClusterCatalogType
			catalogName = template.ClusterCatalogID
		} else if template.ProjectCatalogID != "" {
			catalogType = client.ProjectCatalogType
			catalogName = template.ProjectCatalogID
		}

		namespace, name := helmlib.SplitNamespaceAndName(catalogName)
		catalog, err := helmlib.GetCatalog(catalogType, namespace, name, t.CatalogLister, t.ClusterCatalogLister, t.ProjectCatalogLister)
		if err != nil {
			return err
		}

		helm, err := helmlib.New(catalog)
		if err != nil {
			return err
		}

		iconBytes, err = helm.LoadIcon(template.IconFilename, template.Icon)
		if err != nil {
			return err
		}

		t, err := time.Parse(time.RFC3339, template.Created)
		if err != nil {
			return err
		}

		iconReader := bytes.NewReader(iconBytes)
		apiContext.Response.Header().Set("Cache-Control", "private, max-age=604800")
		http.ServeContent(apiContext.Response, apiContext.Request, template.IconFilename, t, iconReader)
		return nil
	default:
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
}

// templateVersionForRancherVersion indicates if a templateVersion works with the rancher server version
// In the error case it will always return true - if a template is actually invalid for that rancher version
// API validation will handle the rejection
func (t TemplateWrapper) templateVersionForRancherVersion(apiContext *types.APIContext, externalID string) bool {
	var rancherVersion string
	for query, fields := range apiContext.Query {
		if query == "rancherVersion" {
			rancherVersion = fields[0]
		}
	}

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
