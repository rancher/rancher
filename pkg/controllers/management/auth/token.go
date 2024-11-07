package auth

import (
	tokenUtil "github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/features"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	tokenController = "mgmt-auth-tokens-controller"
)

type TokenController struct {
	tokens               v3.TokenInterface
	userAttributes       v3.UserAttributeInterface
	userAttributesLister v3.UserAttributeLister
}

func newTokenController(mgmt *config.ManagementContext) *TokenController {
	n := &TokenController{
		tokens:               mgmt.Management.Tokens(""),
		userAttributes:       mgmt.Management.UserAttributes(""),
		userAttributesLister: mgmt.Management.UserAttributes("").Controller().Lister(),
	}
	return n
}

// sync is called periodically and on real updates
func (t *TokenController) sync(key string, obj *v3.Token) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}
	// remove legacy finalizers
	if obj.DeletionTimestamp != nil {
		finalizers := obj.GetFinalizers()
		newObj := obj.DeepCopy()
		for i, finalizer := range finalizers {
			if finalizer == "controller.cattle.io/cat-token-controller" {
				finalizers = append(finalizers[:i], finalizers[i+1:]...)
				newObj = obj.DeepCopy()
				newObj.SetFinalizers(finalizers)
				var err error
				newObj, err = t.tokens.Update(newObj)
				if err != nil {
					return obj, err
				}
				obj = newObj
				break
			}
		}
	}

	if obj.TTLMillis != 0 && obj.ExpiresAt == "" {
		//compute and save expiresAt
		newObj := obj.DeepCopy()
		var err error
		tokenUtil.SetTokenExpiresAt(newObj)
		if newObj, err = t.tokens.Update(newObj); err != nil {
			return obj, err
		}
		obj = newObj
	}

	// trigger corresponding UserAttribute resource to refresh if token potentially
	// provides new information that is missing from the UserAttribute resource
	refreshUserAttributes, err := t.userAttributesNeedsRefresh(obj.UserID)
	if err != nil {
		return obj, err
	}

	if refreshUserAttributes {
		if err = t.triggerUserAttributesRefresh(obj.UserID); err != nil {
			return obj, err
		}
	}

	// DO NOT remove until tokenHashing is always
	// expected. Anything below this will only execute
	// if tokenHashing is enabled
	if !features.TokenHashing.Enabled() {
		return obj, nil
	}

	if obj.Annotations[tokenUtil.TokenHashed] != "true" {
		newObj := obj.DeepCopy()
		err := tokenUtil.ConvertTokenKeyToHash(newObj)
		if err != nil {
			return obj, err
		}
		if _, err := t.tokens.Update(newObj); err != nil {
			return obj, err
		}
		obj = newObj
	}

	return obj, nil
}

func (t *TokenController) userAttributesNeedsRefresh(user string) (bool, error) {
	if user == "" {
		return false, nil
	}

	userAttribute, err := t.userAttributesLister.Get("", user)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return userAttribute.ExtraByProvider == nil, nil
}

func (t *TokenController) triggerUserAttributesRefresh(user string) error {
	userAttribute, err := t.userAttributesLister.Get("", user)
	if err != nil {
		return err
	}

	userAttribute.NeedsRefresh = true
	_, err = t.userAttributes.Update(userAttribute)
	return err
}
