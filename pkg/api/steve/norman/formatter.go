package norman

import (
	"fmt"
	"strings"

	"github.com/rancher/apiserver/pkg/types"
	types2 "github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/urlbuilder"
	v3 "github.com/rancher/rancher/pkg/schemas/cluster.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
)

type LinksAndActionsFormatter struct {
	multiClusterManager wrangler.MultiClusterManager
	apiVersion          types2.APIVersion
	schemaID            string
}

func NewLinksAndActionsFormatter(multiClusterManager wrangler.MultiClusterManager, apiVersion types2.APIVersion,
	schemaID string) types.Formatter {
	formatter := LinksAndActionsFormatter{
		multiClusterManager: multiClusterManager,
		apiVersion:          apiVersion,
		schemaID:            schemaID,
	}
	return formatter.Formatter
}

func (a *LinksAndActionsFormatter) Formatter(request *types.APIRequest, resource *types.RawResource) {
	schemas := a.multiClusterManager.NormanSchemas()
	if schemas == nil {
		return
	}

	schema := schemas.Schema(&a.apiVersion, a.schemaID)
	if schema == nil {
		logrus.Errorf("failed to find schema %s in %v", a.schemaID, a.apiVersion)
		return
	}

	data, err := convert.EncodeToMap(resource.APIObject.Object)
	if err != nil {
		logrus.Errorf("failed to json encode api object: %v", err)
		return
	}
	schema.Mapper.FromInternal(data)

	normanResource := &types2.RawResource{
		ID:      resource.APIObject.ID,
		Type:    "cluster",
		Schema:  schema,
		Links:   map[string]string{},
		Actions: map[string]string{},
		Values:  data,
	}
	normanRequest := types2.NewAPIContext(request.Request, request.Response, schemas)
	normanRequest.URLBuilder, err = urlbuilder.New(request.Request, v3.Version, schemas)
	normanRequest.AccessControl = accessControlWrapper{
		ac:         request.AccessControl,
		apiRequest: request,
	}
	if err != nil {
		logrus.Errorf("failed to create url builder: %v", err)
		return
	}

	schema.Formatter(normanRequest, normanResource)
	for k, v := range normanResource.Links {
		resource.Links[k] = v
	}
	for k, v := range normanResource.Actions {
		resource.Actions[k] = v
	}
}

type accessControlWrapper struct {
	ac         types.AccessControl
	apiRequest *types.APIRequest
}

func (a accessControlWrapper) CanDo(apiGroup, resource, verb string, apiContext *types2.APIContext, obj map[string]interface{}, schema *types2.Schema) error {
	name, namespace := getNameAndNS(obj)
	// The access control used by this function (schema based - provided by rancher/APIserver), expects the resource in
	// the below format. We re-format it here since the original format does not match what is expected by the api server
	formattedResource := fmt.Sprintf("%s/%s", apiGroup, resource)
	return a.ac.CanDo(a.apiRequest, formattedResource, verb, namespace, name)
}

func getNameAndNS(obj map[string]interface{}) (string, string) {
	var id string
	var namespace string

	if obj != nil {
		id, _ = obj["id"].(string)
		namespace, _ = obj["namespaceId"].(string)
		if namespace == "" {
			pieces := strings.Split(id, ":")
			if len(pieces) == 2 {
				namespace = pieces[0]
			}
		}
	}

	id = strings.TrimPrefix(id, namespace+":")
	return id, namespace
}

func (a accessControlWrapper) CanCreate(apiContext *types2.APIContext, schema *types2.Schema) error {
	panic("not implemented")
}
func (a accessControlWrapper) CanGet(apiContext *types2.APIContext, schema *types2.Schema) error {
	panic("not implemented")
}
func (a accessControlWrapper) CanList(apiContext *types2.APIContext, schema *types2.Schema) error {
	panic("not implemented")
}
func (a accessControlWrapper) CanUpdate(apiContext *types2.APIContext, obj map[string]interface{}, schema *types2.Schema) error {
	panic("not implemented")
}
func (a accessControlWrapper) CanDelete(apiContext *types2.APIContext, obj map[string]interface{}, schema *types2.Schema) error {
	panic("not implemented")
}
func (a accessControlWrapper) Filter(apiContext *types2.APIContext, schema *types2.Schema, obj map[string]interface{}, context map[string]string) map[string]interface{} {
	panic("not implemented")
}
func (a accessControlWrapper) FilterList(apiContext *types2.APIContext, schema *types2.Schema, obj []map[string]interface{}, context map[string]string) []map[string]interface{} {
	panic("not implemented")
}
