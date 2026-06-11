package auth

import (
	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
)

const (
	extTokenController = "mgmt-auth-ext-tokens-controller"
)

type ExtTokenController struct {
	userAttrRefresher UserAttributeRefresher
}

func newExtTokenController(mgmt *config.ManagementContext) *ExtTokenController {
	return &ExtTokenController{
		userAttrRefresher: newUserAttributeRefresher(mgmt),
	}
}

// onChange is called periodically and on real updates
func (t *ExtTokenController) onChange(key string, obj *ext.Token) (*ext.Token, error) {
	if obj == nil {
		return nil, nil
	}
	// remove legacy finalizers
	if obj.DeletionTimestamp != nil {
		return nil, nil
	}

	// trigger corresponding UserAttribute resource to refresh if token potentially
	// provides new information that is missing from the UserAttribute resource
	if err := t.userAttrRefresher.CheckAndRefresh(obj.GetUserID()); err != nil {
		return obj, err
	}

	return obj, nil
}
