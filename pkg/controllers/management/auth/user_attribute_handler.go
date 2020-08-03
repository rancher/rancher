package auth

import (
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	userAttributeController = "mgmt-auth-userattributes-controller"
)

type UserAttributeController struct {
	userAttributes v3.UserAttributeInterface
}

func newUserAttributeController(mgmt *config.ManagementContext) *UserAttributeController {
	ua := &UserAttributeController{
		userAttributes: mgmt.Management.UserAttributes(""),
	}
	return ua
}

//sync is called periodically and on real updates
func (ua *UserAttributeController) sync(key string, obj *v3.UserAttribute) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	if !obj.NeedsRefresh {
		return obj, nil
	}

	obj, err := providerrefresh.RefreshAttributes(obj)
	if err != nil {
		return nil, err
	}

	updated, err := ua.userAttributes.Update(obj)
	if err != nil {
		return nil, err
	}

	return updated, nil
}
