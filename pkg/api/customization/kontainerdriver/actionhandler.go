package kontainerdriver

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type ActionHandler struct {
	KontainerDrivers      v3.KontainerDriverInterface
	KontainerDriverLister v3.KontainerDriverLister
}

func (a ActionHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	switch actionName {
	case "activate":
		return a.activate(apiContext)
	case "deactivate":
		return a.deactivate(apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (a ActionHandler) activate(apiContext *types.APIContext) error {
	return a.setDriverActiveStatus(apiContext, true)
}

func (a ActionHandler) deactivate(apiContext *types.APIContext) error {
	return a.setDriverActiveStatus(apiContext, false)
}

func (a ActionHandler) setDriverActiveStatus(apiContext *types.APIContext, status bool) error {
	driver, err := a.KontainerDriverLister.Get("", apiContext.ID)
	if err != nil {
		return err
	}

	driver.Spec.Active = status

	_, err = a.KontainerDrivers.Update(driver)

	return err
}
