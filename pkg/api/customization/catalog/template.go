package catalog

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/templatecontent"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	var prjCatalogName, clusterCatalogName string
	// version links
	resource.Values["versionLinks"] = extractVersionLinks(apiContext, resource)

	//icon
	delete(resource.Values, "icon")
	resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)

	val := resource.Values
	if val[client.TemplateFieldCatalogID] != nil {
		//catalog link
		catalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.CatalogType)
		catalogName := strings.Split(resource.ID, "-")[0]
		resource.Links["catalog"] = apiContext.URLBuilder.ResourceLinkByID(catalogSchema, catalogName)
	}

	if val[client.TemplateFieldProjectCatalogID] != nil {
		prjCatID, ok := val[client.TemplateFieldProjectCatalogID].(string)
		if ok {
			prjCatalogName = prjCatID
		}
		//project catalog link
		prjCatalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.ProjectCatalogType)
		resource.Links["projectCatalog"] = apiContext.URLBuilder.ResourceLinkByID(prjCatalogSchema, prjCatalogName)
	}

	if val[client.TemplateFieldClusterCatalogID] != nil {
		clusterCatID, ok := val[client.TemplateFieldClusterCatalogID].(string)
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

type TemplateWrapper struct {
	TemplateContentClient v3.TemplateContentInterface
}

func (t TemplateWrapper) TemplateIconHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "icon":
		template := &client.Template{}
		if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, template); err != nil {
			return err
		}

		data, err := templatecontent.GetTemplateFromTag(template.Icon, t.TemplateContentClient)
		if err != nil {
			return err
		}
		t, err := time.Parse(time.RFC3339, template.Created)
		if err != nil {
			return err
		}
		value, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return err
		}
		iconReader := bytes.NewReader(value)
		apiContext.Response.Header().Set("Cache-Control", "private, max-age=604800")
		http.ServeContent(apiContext.Response, apiContext.Request, template.IconFilename, t, iconReader)
		return nil
	default:
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
}
