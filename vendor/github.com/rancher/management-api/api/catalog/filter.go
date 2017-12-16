package catalog

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"time"

	"fmt"

	"strings"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	delete(resource.Values, "icon")
	resource.Values["versionLinks"] = extractVersionLinks(apiContext, resource)
	delete(resource.Values, "versions")
	resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)
	catalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.CatalogType)
	catalogName := strings.Split(resource.ID, "-")[0]
	resource.Links["catalog"] = apiContext.URLBuilder.ResourceLinkByID(catalogSchema, catalogName)
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.Actions["refresh"] = apiContext.URLBuilder.Action("refresh", resource)
}

func RefreshActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	if actionName != "refresh" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	store := apiContext.Schema.Store

	data, err := store.ByID(apiContext, apiContext.Schema, apiContext.ID)
	if err != nil {
		return err
	}
	data["lastRefreshTimestamp"] = time.Now().Format(time.RFC3339)

	_, err = store.Update(apiContext, apiContext.Schema, data, apiContext.ID)
	if err != nil {
		return err
	}
	return nil
}

func extractVersionLinks(apiContext *types.APIContext, resource *types.RawResource) map[string]string {
	schema := apiContext.Schemas.Schema(&managementschema.Version, client.TemplateVersionType)
	versionMap := resource.Values["versions"].([]interface{})
	r := map[string]string{}
	for _, version := range versionMap {
		revision := version.(map[string]interface{})["revision"].(int64)
		version := version.(map[string]interface{})["version"].(string)
		versionID := fmt.Sprintf("%v-%v", resource.ID, revision)
		r[version] = apiContext.URLBuilder.ResourceLinkByID(schema, versionID)
	}
	return r
}

func TemplateIconHandler(apiContext *types.APIContext) error {
	if apiContext.Link != "icon" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	template := &client.Template{}
	if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, template); err != nil {
		return err
	}

	icon, err := base64.StdEncoding.DecodeString(template.Icon)
	if err != nil {
		return err
	}
	iconReader := bytes.NewReader(icon)
	http.ServeContent(apiContext.Response, apiContext.Request, template.IconFilename, time.Time{}, iconReader)

	return nil
}
