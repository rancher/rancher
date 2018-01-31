package authn

import (
	"github.com/rancher/norman/types"
)

func GithubConfigFormatter(apiContext *types.APIContext, resource *types.RawResource) {
	resource.AddAction(apiContext, "configureTest")
	resource.AddAction(apiContext, "testAndApply")
}

func GithubConfigActionHandler(actionName string, action *types.Action, request *types.APIContext) error {
	if actionName == "configureTest" {
		return GithubConfigureTest(actionName, action, request)
	} else if actionName == "testAndApply" {
		return GithubConfigTestApply(actionName, action, request)
	}

	return nil
}

func GithubConfigureTest(actionName string, action *types.Action, request *types.APIContext) error {
	return nil
}

func GithubConfigTestApply(actionName string, action *types.Action, request *types.APIContext) error {
	return nil
}
