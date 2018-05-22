package catalog

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/ghodss/yaml"
	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/templatecontent"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/compose"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "refresh")
	resource.Links["exportYaml"] = apiContext.URLBuilder.Link("exportYaml", resource)
}

func CollectionFormatter(request *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(request, "refresh")
}

type ActionHandler struct {
	CatalogClient v3.CatalogInterface
}

func (a ActionHandler) RefreshActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	if actionName != "refresh" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	catalogs := []v3.Catalog{}
	if apiContext.ID != "" {
		catalog, err := a.CatalogClient.Get(apiContext.ID, metav1.GetOptions{})
		if err != nil {
			return err
		}
		catalogs = append(catalogs, *catalog)
	} else {
		catalogList, err := a.CatalogClient.List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, catalog := range catalogList.Items {
			catalogs = append(catalogs, catalog)
		}
	}
	for _, catalog := range catalogs {
		catalog.Status.LastRefreshTimestamp = time.Now().Format(time.RFC3339)
		v3.CatalogConditionRefreshed.Unknown(&catalog)
		if _, err := a.CatalogClient.Update(&catalog); err != nil {
			return err
		}
	}
	return nil
}

func (a ActionHandler) ExportYamlHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	switch apiContext.Link {
	case "exportyaml":
		catalog, err := a.CatalogClient.Get(apiContext.ID, metav1.GetOptions{})
		if err != nil {
			return rpctypes.ErrGRPCStopped
		}
		topkey := compose.Config{}
		topkey.Version = "v3"
		ca := client.Catalog{}
		if err := convert.ToObj(catalog.Spec, &ca); err != nil {
			return err
		}
		topkey.Catalogs = map[string]client.Catalog{}
		topkey.Catalogs[catalog.Name] = ca
		m, err := convert.EncodeToMap(topkey)
		if err != nil {
			return err
		}
		delete(m["catalogs"].(map[string]interface{})[catalog.Name].(map[string]interface{}), "actions")
		delete(m["catalogs"].(map[string]interface{})[catalog.Name].(map[string]interface{}), "links")
		data, err := json.Marshal(m)
		if err != nil {
			return err
		}

		buf, err := yaml.JSONToYAML(data)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(buf)
		apiContext.Response.Header().Set("Content-Type", "text/yaml")
		http.ServeContent(apiContext.Response, apiContext.Request, "exportYaml", time.Now(), reader)
		return nil
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
