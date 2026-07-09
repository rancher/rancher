package setting

import (
	"fmt"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
)

var ReadOnlySettings = []string{
	"cacerts",
}

func Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	if convert.ToString(resource.Values["source"]) == "env" {
		delete(resource.Links, "update")
	} else if slice.ContainsString(ReadOnlySettings, resource.ID) {
		delete(resource.Links, "update")
	} else {
		setting := map[string]interface{}{
			"id": apiContext.ID,
		}
		if err := apiContext.AccessControl.CanDo(v3.SettingGroupVersionKind.Group, v3.SettingResource.Name, "update", apiContext, setting, apiContext.Schema); err != nil {
			delete(resource.Links, "update")
		}
	}
}

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var setting v3client.Setting

	// request.ID is taken from the request request url, it is possible that the request url does not contain the id
	id := request.ID
	if name, ok := data["name"].(string); ok && id == "" {
		id = name
	}

	if err := access.ByID(request, request.Version, v3client.SettingType, id, &setting); err != nil {
		if !httperror.IsNotFound(err) {
			return err
		}
	}
	if setting.Source == "env" {
		return httperror.NewAPIError(httperror.MethodNotAllowed, fmt.Sprintf("%s is readOnly because its value is from environment variable", id))
	} else if slice.ContainsString(ReadOnlySettings, id) {
		return httperror.NewAPIError(httperror.MethodNotAllowed, fmt.Sprintf("%s is readOnly", id))
	}

	newValue, ok := data["value"]
	if !ok {
		return fmt.Errorf("value not found")
	}
	newValueString, ok := newValue.(string)
	if !ok {
		return fmt.Errorf("value not string")
	}

	var err error
	switch id {
	case "auth-user-info-max-age-seconds":
		_, err = providerrefresh.ParseMaxAge(newValueString)
	case "auth-user-info-resync-cron":
		_, err = providerrefresh.ParseCron(newValueString)
	}

	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	return nil
}
