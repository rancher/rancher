package catalog

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/templatecontent"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
)

type TemplateVerionFormatterWrapper struct {
	TemplateContentClient v3.TemplateContentInterface
}

func (t TemplateVerionFormatterWrapper) TemplateVersionFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	// files
	files := resource.Values["files"]
	delete(resource.Values, "files")
	fileMap := map[string]string{}
	m, ok := files.(map[string]interface{})
	if ok {
		for k, v := range m {
			tag := convert.ToString(v)
			data, err := templatecontent.GetTemplateFromTag(tag, t.TemplateContentClient)
			if err != nil {
				continue
			}
			fileMap[k] = base64.StdEncoding.EncodeToString([]byte(data))
		}
	}
	resource.Values["files"] = fileMap

	// readme
	delete(resource.Values, "readme")
	resource.Links["readme"] = apiContext.URLBuilder.Link("readme", resource)

	// app-readme
	if _, ok := resource.Values["appReadme"]; ok {
		if convert.ToString(resource.Values["appReadme"]) != "" {
			resource.Links["app-readme"] = apiContext.URLBuilder.Link("app-readme", resource)
		}
		delete(resource.Values, "appReadme")
	}

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

func (t TemplateVerionFormatterWrapper) TemplateVersionReadmeHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "readme":
		templateVersion := &client.TemplateVersion{}
		if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, templateVersion); err != nil {
			return err
		}
		data, err := templatecontent.GetTemplateFromTag(templateVersion.Readme, t.TemplateContentClient)
		if err != nil {
			return err
		}
		readmeReader := bytes.NewReader([]byte(data))
		t, err := time.Parse(time.RFC3339, templateVersion.Created)
		if err != nil {
			return err
		}
		apiContext.Response.Header().Set("Content-Type", "text/plain")
		http.ServeContent(apiContext.Response, apiContext.Request, "readme", t, readmeReader)
		return nil
	case "app-readme":
		templateVersion := &client.TemplateVersion{}
		if err := access.ByID(apiContext, apiContext.Version, apiContext.Type, apiContext.ID, templateVersion); err != nil {
			return err
		}
		data, err := templatecontent.GetTemplateFromTag(templateVersion.AppReadme, t.TemplateContentClient)
		if err != nil {
			return err
		}
		readmeReader := bytes.NewReader([]byte(data))
		t, err := time.Parse(time.RFC3339, templateVersion.Created)
		if err != nil {
			return err
		}
		apiContext.Response.Header().Set("Content-Type", "text/plain")
		http.ServeContent(apiContext.Response, apiContext.Request, "app-readme", t, readmeReader)
		return nil
	default:
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
}
