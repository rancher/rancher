package catalog

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"time"

	"fmt"

	"strings"

	"strconv"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

func TemplateFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	// version links
	resource.Values["versionLinks"] = extractVersionLinks(apiContext, resource)

	//icon
	delete(resource.Values, "icon")
	resource.Links["icon"] = apiContext.URLBuilder.Link("icon", resource)

	//catalog link
	catalogSchema := apiContext.Schemas.Schema(&managementschema.Version, client.CatalogType)
	catalogName := strings.Split(resource.ID, "-")[0]
	resource.Links["catalog"] = apiContext.URLBuilder.ResourceLinkByID(catalogSchema, catalogName)

	// delete category
	delete(resource.Values, "category")

	// delete versions
	delete(resource.Values, "versions")
}

func TemplateVersionFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	// files
	files := resource.Values["files"]
	delete(resource.Values, "files")
	fileMap := map[string]string{}
	for _, file := range files.([]interface{}) {
		m := file.(map[string]interface{})
		fileMap[m["name"].(string)] = m["contents"].(string)
	}
	resource.Values["files"] = fileMap

	// readme
	delete(resource.Values, "readme")
	resource.Links["readme"] = apiContext.URLBuilder.Link("readme", resource)

	version := resource.Values["version"].(string)
	if revision, ok := resource.Values["revision"]; ok {
		version = strconv.FormatInt(revision.(int64), 10)
	}
	templateID := strings.TrimSuffix(resource.ID, "-"+version)
	templateSchema := apiContext.Schemas.Schema(&managementschema.Version, client.TemplateType)
	resource.Links["template"] = apiContext.URLBuilder.ResourceLinkByID(templateSchema, templateID)

	upgradeLinks, ok := resource.Values["upgradeVersionLinks"].(map[string]interface{})
	if ok {
		linkMap := map[string]string{}
		templateVersionSchema := apiContext.Schemas.Schema(&managementschema.Version, client.TemplateVersionType)
		for v, versionID := range upgradeLinks {
			linkMap[v] = apiContext.URLBuilder.ResourceLinkByID(templateVersionSchema, versionID.(string))
		}
		delete(resource.Values, "upgradeVersionLinks")
		resource.Values["upgradeVersionLinks"] = linkMap
	}
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "refresh")
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
	r := map[string]string{}
	versionMap, ok := resource.Values["versions"].([]interface{})
	if ok {
		for _, version := range versionMap {
			revision := ""
			if v, ok := version.(map[string]interface{})["revision"].(int64); ok {
				revision = strconv.FormatInt(v, 10)
			}
			version := version.(map[string]interface{})["version"].(string)
			versionID := fmt.Sprintf("%v-%v", resource.ID, version)
			if revision != "" {
				versionID = fmt.Sprintf("%v-%v", resource.ID, revision)
			}
			r[version] = apiContext.URLBuilder.ResourceLinkByID(schema, versionID)
		}
	}
	return r
}

func TemplateIconHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "icon":
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
	default:
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
}

func TemplateVersionReadmeHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "readme":
		templateVersion := &client.TemplateVersion{}
		if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, templateVersion); err != nil {
			return err
		}
		iconReader := bytes.NewReader([]byte(templateVersion.Readme))
		http.ServeContent(apiContext.Response, apiContext.Request, "readme", time.Time{}, iconReader)
		return nil
	default:
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
}
