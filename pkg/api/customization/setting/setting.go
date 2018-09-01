package setting

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	managementschema "github.com/rancher/types/apis/management.cattle.io/v3/schema"
	"github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
)

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	v, ok := resource.Values["value"]
	if !ok || v == "" {
		resource.Values["value"] = resource.Values["default"]
		resource.Values["customized"] = false
	} else {
		resource.Values["customized"] = true
	}
}

type Handler struct{}

func (h *Handler) UpdateHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	if apiContext.ID != "cluster-defaults" {
		return handler.UpdateHandler(apiContext, next)
	}

	store := apiContext.Schema.Store
	if store == nil {
		return httperror.NewAPIError(httperror.NotFound, "no store found")
	}

	data, err := handler.ParseAndValidateBody(apiContext, false)
	if err != nil {
		return err
	}

	value := convert.ToString(data["value"])
	clusterSchema := apiContext.Schemas.Schema(&managementschema.Version, client.ClusterType)
	if value == "" {
		SetClusterDefaults(clusterSchema, apiContext.Schemas)
	} else {
		spec := v3.ClusterSpec{}
		err = json.Unmarshal([]byte(value), &spec)
		if err != nil {
			return fmt.Errorf("unmarshal error %v", err)
		}

		dataMap := map[string]interface{}{}
		err = json.Unmarshal([]byte(value), &dataMap)
		if err != nil {
			return fmt.Errorf("unmarshal error %v", err)
		}
		modify(clusterSchema, dataMap, apiContext.Schemas, getIgnoredFields(apiContext.Schemas))
	}

	data, err = store.Update(apiContext, apiContext.Schema, data, apiContext.ID)
	if err != nil {
		return err
	}

	apiContext.WriteResponse(http.StatusOK, data)
	return nil
}

func modify(schema *types.Schema, data map[string]interface{}, schemas *types.Schemas, toIgnore map[string]bool) {
	for name, value := range data {
		if _, ok := toIgnore[name]; ok {
			continue
		}
		if field, ok := schema.ResourceFields[name]; ok {
			checkSchema := schemas.Schema(&managementschema.Version, field.Type)
			if checkSchema != nil {
				modify(checkSchema, convert.ToMapInterface(value), schemas, toIgnore)
			} else {
				field.Default = value
				schema.ResourceFields[name] = field
			}
		}
	}
}

func ModifySchema(schema *types.Schema, schemas *types.Schemas) {
	data := settings.ClusterDefaults.Get()
	if data != "" {
		dataMap := map[string]interface{}{}
		err := json.Unmarshal([]byte(data), &dataMap)
		if err != nil {
			return
		}
		modify(schema, dataMap, schemas, getIgnoredFields(schemas))
	}
}

func SetClusterDefaults(schema *types.Schema, schemas *types.Schemas) {
	ans, err := json.Marshal(getClusterSpec(schema, schemas, getIgnoredFields(schemas)))
	if err != nil {
		logrus.Warnf("error setting cluster defaults %v", err)
	}
	settings.ClusterDefaults.Set(string(ans))
}

func getClusterSpec(schema *types.Schema, schemas *types.Schemas, toIgnore map[string]bool) map[string]interface{} {
	data := map[string]interface{}{}
	for name, field := range schema.ResourceFields {
		if _, ok := toIgnore[name]; ok {
			continue
		}
		checkSchema := schemas.Schema(&managementschema.Version, field.Type)
		if checkSchema != nil {
			value := getClusterSpec(checkSchema, schemas, toIgnore)
			if len(value) > 0 {
				data[name] = value
			}
		} else {
			data[name] = field.Default
		}
	}
	return data
}

func getIgnoredFields(schemas *types.Schemas) map[string]bool {
	ignored := map[string]bool{}
	clusterSchema := schemas.Schema(&managementschema.Version, client.ClusterType)
	specSchema := schemas.Schema(&managementschema.Version, client.ClusterSpecType)
	statusSchema := schemas.Schema(&managementschema.Version, client.ClusterStatusType)
	for name := range statusSchema.ResourceFields {
		ignored[name] = true
	}
	for name := range clusterSchema.ResourceFields {
		if strings.HasSuffix(name, "Config") && !strings.HasPrefix(name, "rancher") {
			ignored[name] = true
		}
		if _, ok := specSchema.ResourceFields[name]; !ok {
			ignored[name] = true
		}
	}
	ignored["clusterName"] = true
	return ignored
}
