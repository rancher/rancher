package auth

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/userretention"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
)

const (
	userAttributeController = "mgmt-auth-userattributes-controller"
)

type UserAttributeController struct {
	userAttributes            mgmtcontrollers.UserAttributeClient
	providerRefresh           func(attribs *apiv3.UserAttribute) (*apiv3.UserAttribute, error)
	ensureUserRetentionLabels func(attribs *apiv3.UserAttribute) error
}

func newUserAttributeController(mgmt *config.ManagementContext) *UserAttributeController {
	userretentionLabeler := userretention.NewUserLabeler(context.Background(), mgmt.Wrangler)

	return &UserAttributeController{
		userAttributes:            mgmt.Wrangler.Mgmt.UserAttribute(),
		providerRefresh:           providerrefresh.RefreshAttributes,
		ensureUserRetentionLabels: userretentionLabeler.EnsureForAttributes,
	}
}

// sync is called periodically and on real updates
func (c *UserAttributeController) sync(key string, attribs *apiv3.UserAttribute) (runtime.Object, error) {
	if attribs == nil || attribs.DeletionTimestamp != nil {
		return nil, nil
	}

	// Preserve the name as attribs can be set to nil by the following calls.
	name := attribs.Name

	err := c.ensureUserRetentionLabels(attribs)
	if err != nil {
		return nil, fmt.Errorf("error setting user retention labels for user %s: %w", name, err)
	}

	if !attribs.NeedsRefresh {
		return attribs, nil
	}

	// We want to avoid mutiple provider refresh calls as it's a very expensive operation
	// that caused issues in the past. To avoid this we:
	// 1. Re-fetch the object and recheck NeedsRefresh before proceeding with refresh
	//    as it's possible that it's already false by the time RefreshAttributes finishes
	//    e.g. if the user logins while refresh is running.
	// 2. Explicitly handle the update conflict and carry over the RefreshAttributes changes
	//    to a fresh state of the object and attempt to update it one more time.
	// This is a temporary measure to make sure capturing last login time doesn't makes things worse.
	// We want to move away from this pattern of triggering a refresh by using a field (NeedsRefresh)
	// on the resource object itself, which is inherently racey.
	// Instead we plan to have a dedicated CRD for triggering refreshes.
	attribs, err = c.userAttributes.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting user attribute %s before provider refresh: %w", name, err)
	}
	if !attribs.NeedsRefresh {
		return attribs, nil
	}

	if _, ok := attribs.Annotations[common.ProviderRefreshErrorAnnotation]; ok {
		logrus.Debugf("Skipping provider refresh for %s: annotated with non-transient error", name)
		attribs.NeedsRefresh = false
		updated, uerr := c.userAttributes.Update(attribs)
		if uerr != nil {
			return nil, fmt.Errorf("error clearing NeedsRefresh for skipped user %s: %w", name, uerr)
		}
		return updated, nil
	}

	// The copy read before the refresh tells, on conflict, which values the
	// refresh changed.
	beforeRefresh := attribs.DeepCopy()

	attribs, err = c.providerRefresh(attribs)
	if err != nil {
		var nte *common.NonTransientError
		if errors.As(err, &nte) {
			return c.skipRefresh(name, err)
		}

		return nil, fmt.Errorf("error refreshing user attribute %s: %w", name, err)
	}

	updated, err := c.userAttributes.Update(attribs)
	if err == nil {
		return updated, nil
	}

	if apierrors.IsRequestEntityTooLargeError(err) {
		return c.skipRefresh(name, err)
	}

	// We deliberately wrap and shadow the original error so that we can return it later on.
	// IsConflict is still able to figure out if it's a conflict.
	err = fmt.Errorf("error updating user attribute %s after provider refresh: %w", name, err)
	if !apierrors.IsConflict(err) {
		return nil, err
	}

	newAttribs, nerr := c.userAttributes.Get(name, metav1.GetOptions{})
	if nerr != nil {
		logrus.Errorf("error getting new version of user attribute %s: %v", name, nerr)
		return nil, err // Deliberately return the original error.
	}

	newAttribs.NeedsRefresh = attribs.NeedsRefresh
	newAttribs.LastRefresh = attribs.LastRefresh
	applyRefreshChanges(beforeRefresh, attribs, newAttribs)

	updated, nerr = c.userAttributes.Update(newAttribs)
	if nerr != nil {
		logrus.Errorf("error updating new version of user attribute %s: %v", name, nerr)
		return nil, err // Deliberately return the original error.
	}

	return updated, nil
}

// applyRefreshChanges copies onto current only the group principals and
// extras the refresh changed from before. Values the refresh left alone keep
// what current has, so writes made while the refresh ran, for example a SCIM
// group removal, aren't undone. A provider's group list counts as changed when
// its set of principal names differs; extras are compared key by key. An
// entry the refresh added counts as changed even when it's empty.
func applyRefreshChanges(before, refreshed, current *apiv3.UserAttribute) {
	for provider, groups := range refreshed.GroupPrincipals {
		old, existed := before.GroupPrincipals[provider]
		if existed && samePrincipalNames(old.Items, groups.Items) {
			continue
		}
		if current.GroupPrincipals == nil {
			current.GroupPrincipals = map[string]apiv3.Principals{}
		}
		current.GroupPrincipals[provider] = groups
	}
	for provider := range before.GroupPrincipals {
		if _, ok := refreshed.GroupPrincipals[provider]; !ok {
			delete(current.GroupPrincipals, provider)
		}
	}

	for provider, extra := range refreshed.ExtraByProvider {
		old, existed := before.ExtraByProvider[provider]
		if existed && maps.EqualFunc(old, extra, slices.Equal) {
			continue
		}
		if current.ExtraByProvider == nil {
			current.ExtraByProvider = map[string]map[string][]string{}
		}
		currentExtra, ok := current.ExtraByProvider[provider]
		if !ok {
			// The current object has no entry to merge into. Take the
			// refreshed entry whole, as for groups.
			current.ExtraByProvider[provider] = maps.Clone(extra)
			if current.ExtraByProvider[provider] == nil {
				current.ExtraByProvider[provider] = map[string][]string{}
			}
			continue
		}
		merged := maps.Clone(currentExtra)
		if merged == nil {
			merged = map[string][]string{}
		}
		for key, value := range extra {
			if oldValue, ok := old[key]; !ok || !slices.Equal(oldValue, value) {
				merged[key] = value
			}
		}
		for key := range old {
			if _, ok := extra[key]; !ok {
				delete(merged, key)
			}
		}
		current.ExtraByProvider[provider] = merged
	}
	for provider := range before.ExtraByProvider {
		if _, ok := refreshed.ExtraByProvider[provider]; !ok {
			delete(current.ExtraByProvider, provider)
		}
	}
}

// samePrincipalNames reports whether a and b hold the same set of principal
// names.
func samePrincipalNames(a, b []apiv3.Principal) bool {
	if len(a) != len(b) {
		return false
	}
	names := func(principals []apiv3.Principal) []string {
		out := make([]string, 0, len(principals))
		for _, p := range principals {
			out = append(out, p.Name)
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(names(a), names(b))
}

// skipRefresh annotates the UserAttribute with the error so future refreshes
// are skipped until the user logs in again.
func (c *UserAttributeController) skipRefresh(name string, reason error) (runtime.Object, error) {
	logrus.Warnf("Skipping provider refresh for %s due to non-transient error: %v", name, reason)

	// Re-fetch from the API server inside the retry: gets a clean version
	// (important for the 413 case where the in-memory attribs may contain
	// bloated GroupPrincipals) and keeps the ResourceVersion fresh so the
	// annotation write doesn't lose to a concurrent trigger or consumer
	// writeback under contention.
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		attribs, err := c.userAttributes.Get(name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			// The user attribute was deleted under us.
			// The annotation would have no reader. No point in requeuing.
			return nil
		}
		if err != nil {
			return err
		}
		if attribs.Annotations == nil {
			attribs.Annotations = make(map[string]string)
		}
		attribs.Annotations[common.ProviderRefreshErrorAnnotation] = reason.Error()
		attribs.NeedsRefresh = false
		_, err = c.userAttributes.Update(attribs)
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("error annotating user attribute %s to skip refresh: %w", name, err)
	}

	return nil, nil
}
