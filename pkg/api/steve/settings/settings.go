package settings

import (
	"fmt"
	"slices"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	schema2 "github.com/rancher/steve/pkg/schema"
	steve "github.com/rancher/steve/pkg/server"
	"github.com/rancher/steve/pkg/stores/proxy"
	wranglerdata "github.com/rancher/wrangler/v3/pkg/data"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
)

var ReadOnlySettings = []string{
	"cacerts",
}

type settingsStore struct {
	types.Store
}

func validate(id, source string, data wranglerdata.Object) error {
	if source == "env" {
		return apierror.NewAPIError(validation.MethodNotAllowed, fmt.Sprintf("%s is readOnly because its value is from environment variable", id))
	}

	if slices.Contains(ReadOnlySettings, id) {
		return apierror.NewAPIError(validation.MethodNotAllowed, fmt.Sprintf("%s is readOnly", id))
	}

	newValue, ok := data["value"]
	if !ok {
		return apierror.NewAPIError(validation.InvalidBodyContent, "value not found")
	}

	newValueString, ok := newValue.(string)
	if !ok {
		return apierror.NewAPIError(validation.InvalidBodyContent, "value not string")
	}

	var err error
	switch id {
	case "auth-user-info-max-age-seconds":
		_, err = providerrefresh.ParseMaxAge(newValueString)
	case "auth-user-info-resync-cron":
		_, err = providerrefresh.ParseCron(newValueString)
	}

	if err != nil {
		return apierror.NewAPIError(validation.InvalidBodyContent, err.Error())
	}

	return nil
}

func (s *settingsStore) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	current, err := s.Store.ByID(apiOp, schema, id)
	if err != nil {
		return types.APIObject{}, err
	}

	if err := validate(id, current.Data().String("source"), data.Data()); err != nil {
		return types.APIObject{}, err
	}

	return s.Store.Update(apiOp, schema, data, id)
}

func (s *settingsStore) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	if slices.Contains(ReadOnlySettings, id) {
		return types.APIObject{}, apierror.NewAPIError(validation.MethodNotAllowed, fmt.Sprintf("Cannot delete readOnly setting %s", id))
	}

	return s.Store.Delete(apiOp, schema, id)
}

func Register(server *steve.Server) {
	server.SchemaFactory.AddTemplate(schema2.Template{
		Group: "management.cattle.io",
		Kind:  "Setting",
		Customize: func(schema *types.APISchema) {
			s := proxy.NewProxyStore(server.ClientFactory, nil, server.AccessSetLookup, nil)

			schema.Store = &settingsStore{
				Store: s,
			}
		},
		Formatter: func(request *types.APIRequest, resource *types.RawResource) {
			data := resource.APIObject.Data()
			if data.String("value") == "" {
				data.Set("value", data.String("default"))
			}

			if data.String("source") == "env" || slices.Contains(ReadOnlySettings, resource.ID) {
				delete(resource.Links, "update")
			} else {
				if err := request.AccessControl.CanDo(request, fmt.Sprintf("%s/%s", v3.SettingGroupVersionResource.Group, v3.SettingResource.Name), "update", resource.APIObject.Namespace(), resource.APIObject.Name()); err != nil {
					delete(resource.Links, "update")
				}
			}
		},
	})
}
