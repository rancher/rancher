package catalog

import (
	"bytes"
	"net/http"
	"strings"
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	helmlib "github.com/rancher/rancher/pkg/catalog/helm"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
)

type TemplateWrapper struct {
	CatalogLister                v3.CatalogLister
	ClusterCatalogLister         v3.ClusterCatalogLister
	ProjectCatalogLister         v3.ProjectCatalogLister
	CatalogTemplateVersionLister v3.CatalogTemplateVersionLister
}

func (t TemplateWrapper) TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	var prjCatalogName, clusterCatalogName string

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

func (t TemplateWrapper) TemplateIconHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "icon":
		template := &client.Template{}
		if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, template); err != nil {
			return err
		}
		if template.Icon == "" || strings.HasPrefix(template.Icon, "http:") || strings.HasPrefix(template.Icon, "https:") {
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
		// add security headers (similar to raw.githubusercontent)
		apiContext.Response.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
		apiContext.Response.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(apiContext.Response, apiContext.Request, template.IconFilename, t, iconReader)
		return nil
	default:
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
}
