package catalog

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/ghodss/yaml"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	client "github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/compose"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SystemLibraryURL            = "https://git.rancher.io/system-charts"
	SystemCatalogName           = "system-library"
	embededSystemCatalogSetting = "system-catalog"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if canUpdateCatalog(apiContext, resource) {
		resource.AddAction(apiContext, "refresh")
	}
	resource.Links["exportYaml"] = apiContext.URLBuilder.Link("exportYaml", resource)
	if resource.Values["url"] == SystemLibraryURL && resource.Values["name"] == SystemCatalogName {
		delete(resource.Links, "remove")

		if strings.ToLower(settings.SystemCatalog.Get()) == "bundled" {
			delete(resource.Links, "update")
		}
	}
}

func CollectionFormatter(apiContext *types.APIContext, collection *types.GenericCollection) {
	if canUpdateCatalog(apiContext, nil) {
		collection.AddAction(apiContext, "refresh")
	}
}

type ActionHandler struct {
	CatalogClient        v3.CatalogInterface
	ProjectCatalogClient v3.ProjectCatalogInterface
	ClusterCatalogClient v3.ClusterCatalogInterface
}

func (a ActionHandler) refreshCatalog(catalog *v3.Catalog) (err error) {
	for i := 0; i < 3; i++ {
		catalog, err = a.CatalogClient.Get(catalog.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		catalog.Status.LastRefreshTimestamp = time.Now().Format(time.RFC3339)
		v3.CatalogConditionRefreshed.Unknown(catalog)
		_, err = a.CatalogClient.Update(catalog)
		if err == nil {
			break
		}
	}
	return err
}

func (a ActionHandler) RefreshActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	if actionName != "refresh" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
	if !canUpdateCatalog(apiContext, nil) {
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
	var catalogNames []string
	for _, catalog := range catalogs {
		if err := a.refreshCatalog(&catalog); err != nil {
			return err
		}
		catalogNames = append(catalogNames, catalog.Name)
	}
	data := map[string]interface{}{
		"catalogs": catalogNames,
		"type":     "catalogRefresh",
	}
	apiContext.WriteResponse(http.StatusOK, data)
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
		delete(m["catalogs"].(map[string]interface{})[catalog.Name].(map[string]interface{}), "password")
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

func (a ActionHandler) RefreshProjectCatalogActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	if actionName != "refresh" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
	if !canUpdateCatalog(apiContext, nil) {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	prjCatalogs := []v3.ProjectCatalog{}
	if apiContext.ID != "" {
		ns, name := ref.Parse(apiContext.ID)
		catalog, err := a.ProjectCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		prjCatalogs = append(prjCatalogs, *catalog)
	} else {
		catalogList, err := a.ProjectCatalogClient.List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, catalog := range catalogList.Items {
			prjCatalogs = append(prjCatalogs, catalog)
		}
	}
	var catalogNames []string
	for _, catalog := range prjCatalogs {
		catalog.Status.LastRefreshTimestamp = time.Now().Format(time.RFC3339)
		v3.CatalogConditionRefreshed.Unknown(&catalog)
		if _, err := a.ProjectCatalogClient.Update(&catalog); err != nil {
			return err
		}
	}
	data := map[string]interface{}{
		"catalogs": catalogNames,
		"type":     "catalogRefresh",
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func (a ActionHandler) RefreshClusterCatalogActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	if actionName != "refresh" {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}
	if !canUpdateCatalog(apiContext, nil) {
		return httperror.NewAPIError(httperror.NotFound, "not found")
	}

	clCatalogs := []v3.ClusterCatalog{}
	if apiContext.ID != "" {
		ns, name := ref.Parse(apiContext.ID)
		catalog, err := a.ClusterCatalogClient.GetNamespaced(ns, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		clCatalogs = append(clCatalogs, *catalog)
	} else {
		catalogList, err := a.ClusterCatalogClient.List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, catalog := range catalogList.Items {
			clCatalogs = append(clCatalogs, catalog)
		}
	}
	var catalogNames []string
	for _, catalog := range clCatalogs {
		catalog.Status.LastRefreshTimestamp = time.Now().Format(time.RFC3339)
		v3.CatalogConditionRefreshed.Unknown(&catalog)
		if _, err := a.ClusterCatalogClient.Update(&catalog); err != nil {
			return err
		}
	}
	data := map[string]interface{}{
		"catalogs": catalogNames,
		"type":     "catalogRefresh",
	}
	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func canUpdateCatalog(apiContext *types.APIContext, resource *types.RawResource) bool {
	var groupName, resourceName string
	switch apiContext.Type {
	case client.CatalogType:
		groupName, resourceName = v3.CatalogGroupVersionKind.Group, v3.CatalogResource.Name
	case client.ClusterCatalogType:
		groupName, resourceName = v3.ClusterCatalogGroupVersionKind.Group, v3.ClusterCatalogResource.Name
	case client.ProjectCatalogType:
		groupName, resourceName = v3.ProjectCatalogGroupVersionKind.Group, v3.ProjectCatalogResource.Name
	default:
		return false
	}
	obj := rbac.ObjFromContext(apiContext, resource)
	return apiContext.AccessControl.CanDo(groupName, resourceName, "update", apiContext, obj, apiContext.Schema) == nil
}
