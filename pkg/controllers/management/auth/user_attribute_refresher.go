package auth

import (
	wrangmgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type UserAttributeRefresher struct {
	userAttributes       wrangmgmtv3.UserAttributeController
	userAttributesLister wrangmgmtv3.UserAttributeCache
}

func newUserAttributeRefresher(mgmt *config.ManagementContext) UserAttributeRefresher {
	return UserAttributeRefresher{
		userAttributes:       mgmt.Wrangler.Mgmt.UserAttribute(),
		userAttributesLister: mgmt.Wrangler.Mgmt.UserAttribute().Cache(),
	}
}

// CheckAndRefresh is invoked periodically and on real updates, by the token
// controllers, legacy and ext
func (t *UserAttributeRefresher) CheckAndRefresh(userID string) error {

	refreshUserAttributes, err := t.needsRefresh(userID)
	if err != nil {
		return err
	}

	if !refreshUserAttributes {
		return nil
	}

	return t.triggerRefresh(userID)
}

func (t *UserAttributeRefresher) needsRefresh(user string) (bool, error) {
	if user == "" {
		return false, nil
	}

	userAttribute, err := t.userAttributesLister.Get(user)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return userAttribute.ExtraByProvider == nil, nil
}

func (t *UserAttributeRefresher) triggerRefresh(user string) error {
	// Re-fetch from the API server inside the retry; the lister cache used
	// for the gating check can be stale and lead to 409 conflicts when other
	// writers (e.g. the refresh consumer) touch the object concurrently.
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		userAttribute, err := t.userAttributes.Get(user, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			// The user attribute was deleted under us.
			// Nothing to refresh, no point in requeuing.
			return nil
		}
		if err != nil {
			return err
		}
		if userAttribute.NeedsRefresh {
			return nil
		}
		userAttribute.NeedsRefresh = true
		_, err = t.userAttributes.Update(userAttribute)
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	})
}
