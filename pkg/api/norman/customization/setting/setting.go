package setting

import (
	"fmt"
	"strconv"

	"github.com/rancher/norman/api/access"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
)

const (
	minTTLMinutes         = 30 // 30 minutes
	minGracePeriodMinutes = 10 // 10 minutes
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
	case "crt-default-ttl-minutes":
		err = validateCRTTTL(request, newValueString)
	case "crt-default-grace-period-minutes":
		err = validateCRTGracePeriod(request, newValueString)
	}

	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	return nil
}

func validateCRTTTL(request *types.APIContext, value string) error {
	ttl, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("crt-default-ttl-minutes must be a valid integer: %v", err)
	}
	if ttl < minTTLMinutes {
		return fmt.Errorf("crt-default-ttl-minutes must be >= %d", minTTLMinutes)
	}
	// validate TTL > grace period
	gp := settings.CRTDefaultGracePeriod.GetInt()
	if ttl <= gp {
		return fmt.Errorf("crt-default-ttl-minutes (%d) must be greater than crt-default-grace-period-minutes (%d)", ttl, gp)
	}

	return nil
}

func validateCRTGracePeriod(request *types.APIContext, value string) error {
	gp, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("crt-default-grace-period-minutes must be a valid integer: %v", err)
	}
	if gp < minGracePeriodMinutes {
		return fmt.Errorf("crt-default-grace-period-minutes must be >= %d", minGracePeriodMinutes)
	}
	// validate TTL > grace period
	ttl := settings.CRTDefaultTTL.GetInt()
	if ttl <= gp {
		return fmt.Errorf("crt-default-grace-period-minutes (%d) must be less than crt-default-ttl-minutes (%d)", gp, ttl)
	}

	return nil
}
