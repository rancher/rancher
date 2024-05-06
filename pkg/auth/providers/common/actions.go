package common

import (
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
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
			logrus.Infof("Disabling auth provider %s from the action.", authConfigName)
			_, err = authConfigs.ObjectClient().Update(authConfigName, o)
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
