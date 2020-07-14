package auth

import (
	tokenUtil "github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	tokenController = "mgmt-auth-tokens-controller"
)

type TokenController struct {
	tokens v3.TokenInterface
}

func newTokenController(mgmt *config.ManagementContext) *TokenController {
	n := &TokenController{
		tokens: mgmt.Management.Tokens(""),
	}
	return n
}

//sync is called periodically and on real updates
func (n *TokenController) sync(key string, obj *v3.Token) (runtime.Object, error) {
	if obj == nil {
		return nil, nil
	}
	// remove legacy finalizers
	if obj.DeletionTimestamp != nil {
		finalizers := obj.GetFinalizers()
		for i, finalizer := range finalizers {
			if finalizer == "controller.cattle.io/cat-token-controller" {
				finalizers = append(finalizers[:i], finalizers[i+1:]...)
				newObj := obj.DeepCopy()
				newObj.SetFinalizers(finalizers)
				var err error
				obj, err = n.tokens.Update(newObj)
				if err != nil {
					return nil, err
				}
				break
			}
		}
	}

	if obj.TTLMillis != 0 && obj.ExpiresAt == "" {
		//compute and save expiresAt
		newObj := obj.DeepCopy()
		tokenUtil.SetTokenExpiresAt(newObj)
		if _, err := n.tokens.Update(newObj); err != nil {
			return nil, err
		}
	}
	return nil, nil
}
