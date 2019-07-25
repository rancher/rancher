package common

import (
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	client "github.com/rancher/types/client/management/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func HandleCommonAction(actionName string, action *types.Action, request *types.APIContext, authConfigName string, authConfigs v3.AuthConfigInterface) (bool, error) {
	if actionName == "disable" {
		request.Response.Header().Add("Content-type", "application/json")
		o, err := authConfigs.ObjectClient().UnstructuredClient().Get(authConfigName, v1.GetOptions{})
		if err != nil {
			return false, err
		}
		u, _ := o.(runtime.Unstructured)
		config := u.UnstructuredContent()
		if e, ok := config[client.AuthConfigFieldEnabled].(bool); ok && e {
			config[client.AuthConfigFieldEnabled] = false
			retainFields := map[string]bool{"apiVersion": true, "kind": true, "metadata": true, "type": true}
			for field := range config {
				if !retainFields[field] {
					delete(config, field)
				}
			}
			logrus.Infof("Disabling auth provider %v", authConfigName)
			_, err := authConfigs.ObjectClient().Update(authConfigName, o)
			return true, err
		}
	}

	return false, nil
}

func AddCommonActions(apiContext *types.APIContext, resource *types.RawResource) {
	if e, ok := resource.Values["enabled"].(bool); ok && e {
		resource.AddAction(apiContext, "disable")
	}
}
