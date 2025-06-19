package kontainerdriver

import (
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/channelserver"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	kd "github.com/rancher/rancher/pkg/kontainerdrivermetadata"
)

type ActionHandler struct {
	KontainerDrivers      v3.KontainerDriverInterface
	KontainerDriverLister v3.KontainerDriverLister
	MetadataHandler       kd.MetadataController
}

type ListHandler struct {
	ConfigMapLister v1.ConfigMapLister
}

func (a ActionHandler) ActionHandler(actionName string, action *types.Action, apiContext *types.APIContext) error {
	// passing nil as the resource only works because just namespace is grabbed from it and nodedriver is not namespaced
	if err := apiContext.AccessControl.CanDo(v3.KontainerDriverGroupVersionKind.Group, v3.KontainerDriverResource.Name, "update", apiContext, nil, apiContext.Schema); err != nil {
		return err
	}

	switch actionName {
	case "activate":
		return a.activate(apiContext)
	case "deactivate":
		return a.deactivate(apiContext)
	case "refresh":
		return a.refresh(apiContext)
	}
	return httperror.NewAPIError(httperror.NotFound, "not found")
}

func (a ActionHandler) activate(apiContext *types.APIContext) error {
	return a.setDriverActiveStatus(apiContext, true)
}

func (a ActionHandler) deactivate(apiContext *types.APIContext) error {
	return a.setDriverActiveStatus(apiContext, false)
}

func (a ActionHandler) refresh(apiContext *types.APIContext) error {
	response := map[string]interface{}{}

	// refresh to sync k3s/rke2 releases
	channelserver.Refresh()
	apiContext.WriteResponse(http.StatusOK, response)
	return nil
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
