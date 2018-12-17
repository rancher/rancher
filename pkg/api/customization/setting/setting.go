package setting

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
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

func Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	newValue, ok := data["value"]
	if !ok {
		return fmt.Errorf("value not found")
	}
	newValueString, ok := newValue.(string)
	if !ok {
		return fmt.Errorf("value not string")
	}

	var err error
	switch request.ID {
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
