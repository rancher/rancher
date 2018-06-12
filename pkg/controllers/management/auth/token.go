package auth

import (
	tokenUtil "github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
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
func (n *TokenController) sync(key string, obj *v3.Token) error {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil
	}

	if obj.TTLMillis != 0 && obj.ExpiresAt == "" {
		//compute and save expiresAt
		newObj := obj.DeepCopy()
		tokenUtil.SetTokenExpiresAt(newObj)
		if _, err := n.tokens.Update(newObj); err != nil {
			return err
		}
	}
	return nil
}
