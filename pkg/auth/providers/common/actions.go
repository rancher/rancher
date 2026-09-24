package common

import (
	"cmp"
	"encoding/json"

	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// HandleCommonAction handles the common actions for auth providers.
//
// For the "disable" action, it disables the provider if it is currently enabled.
func HandleCommonAction(actionName string, action *types.Action, request *types.APIContext, authConfigName string, authConfigs v3.AuthConfigInterface) (bool, error) {
	if actionName == "disable" {
		// This keeps this function backwards compatible.
		nameFromRequest, err := configNameFromRequest(request)
		if err != nil {
			return false, err
		}
		configName := cmp.Or(nameFromRequest, authConfigName)

		request.Response.Header().Add("Content-type", "application/json")
		o, err := authConfigs.ObjectClient().UnstructuredClient().Get(configName, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		u, _ := o.(runtime.Unstructured)
		config := u.UnstructuredContent()
		if e, ok := config[client.AuthConfigFieldEnabled].(bool); ok && e {
			config[client.AuthConfigFieldEnabled] = false
			logrus.Infof("Disabling auth provider %s from the action.", configName)
			_, err = authConfigs.ObjectClient().Update(configName, o)
			return true, err
		}
	}

	return false, nil
}

// configNameFromRequest extracts the "configName" field from the request body.
//
// It returns an error if the request body is invalid and "" if there's no field.
func configNameFromRequest(request *types.APIContext) (string, error) {
	var body struct {
		ConfigName string `json:"configName"`
	}
	if err := json.NewDecoder(request.Request.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.ConfigName, nil
}

func AddCommonActions(apiContext *types.APIContext, resource *types.RawResource) {
	if e, ok := resource.Values["enabled"].(bool); ok && e {
		resource.AddAction(apiContext, "disable")
	}
}
