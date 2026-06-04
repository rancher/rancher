package providerrefresh

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/settings"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
)

type UserAuthRefresher interface {
	TriggerAllUserRefresh()
	TriggerUserRefresh(string, bool)
}

func NewUserAuthRefresher(scaledContext *config.ScaledContext) UserAuthRefresher {
	extTokenStore := exttokenstore.NewSystemFromWrangler(scaledContext.Wrangler)

	return &refresher{
		tokenLister:               scaledContext.Management.Tokens("").Controller().Lister(),
		tokens:                    scaledContext.Management.Tokens(""),
		tokenMGR:                  tokens.NewManager(scaledContext.Wrangler),
		userLister:                scaledContext.Management.Users("").Controller().Lister(),
		userAttributes:            scaledContext.Management.UserAttributes(""),
		userAttributeLister:       scaledContext.Management.UserAttributes("").Controller().Lister(),
		extTokenStore:             extTokenStore,
		ensureAndGetUserAttribute: scaledContext.UserManager.EnsureAndGetUserAttribute,
	}
}

type refresher struct {
	sync.Mutex
	tokenLister               v3.TokenLister
	tokens                    v3.TokenInterface
	tokenMGR                  *tokens.Manager
	userLister                v3.UserLister
	userAttributes            v3.UserAttributeInterface
	userAttributeLister       v3.UserAttributeLister
	unparsedMaxAge            string
	maxAge                    time.Duration
	extTokenStore             *exttokenstore.SystemStore
	ensureAndGetUserAttribute func(userID string) (*apiv3.UserAttribute, bool, error)
}

func (r *refresher) ensureMaxAgeUpToDate(maxAge string) {
	if r.unparsedMaxAge == maxAge || maxAge == "" {
		return
	}

	parsed, err := ParseMaxAge(maxAge)
	if err != nil {
		logrus.Errorf("Error parsing max age %v", err)
		return
	}
	r.unparsedMaxAge = maxAge
	r.maxAge = parsed
}

func (r *refresher) TriggerUserRefresh(userName string, force bool) {
	if force {
		logrus.Debugf("Triggering auth refresh manually on %v", userName)
	} else {
		logrus.Debugf("Triggering auth refresh on %v", userName)
	}
	r.Lock()
	r.ensureMaxAgeUpToDate(settings.AuthUserInfoMaxAgeSeconds.Get())
	r.Unlock()
	if !force && (r.maxAge <= 0) {
		logrus.Debugf("Skipping refresh trigger on user %v because max age setting is <= 0", userName)
		return
	}

	r.triggerUserRefresh(userName, force)
}

func (r *refresher) TriggerAllUserRefresh() {
	logrus.Debug("Triggering auth refresh manually on all users")
	r.refreshAll(true)
}

func (r *refresher) refreshAll(force bool) {
	users, err := r.userLister.List("", labels.Everything())
	if err != nil {
		logrus.Errorf("Error listing Users during auth provider refresh: %v", err)
	}
	for _, user := range users {
		r.triggerUserRefresh(user.Name, force)
	}
}

func (r *refresher) triggerUserRefresh(userName string, force bool) {
	attribs, needCreate, err := r.ensureAndGetUserAttribute(userName)
	if err != nil {
		logrus.Errorf("Error fetching user attribute to trigger refresh: %v", err)
		return
	}
	now := time.Now().UTC()
	// in the case there is an invalid (or no) last refresh ignore the error, lastrefresh will be 0
	lastRefresh, _ := time.Parse(time.RFC3339, attribs.LastRefresh)
	earliestRefresh := lastRefresh.Add(r.maxAge)
	if !force && now.Before(earliestRefresh) {
		logrus.Debugf("Skipping refresh for %v due to max-age", userName)
		return
	}

	user, err := r.userLister.Get("", userName)
	if err != nil {
		logrus.Errorf("Error finding user before triggering refresh %v", err)
		return
	}

	for _, principalID := range user.PrincipalIDs {
		if strings.HasPrefix(principalID, "system://") {
			logrus.Debugf("Skipping refresh for system-user %v ", userName)
			return
		}
	}

	if _, ok := attribs.Annotations[common.ProviderRefreshErrorAnnotation]; ok {
		logrus.Debugf("Skipping refresh trigger for %v: annotated with non-transient error", userName)
		return
	}

	if needCreate {
		attribs.NeedsRefresh = true
		if _, err := r.userAttributes.Create(attribs); err != nil {
			logrus.Errorf("Error creating user attribute to trigger refresh: %v", err)
		}
		return
	}

	// Re-fetch from the API server (not the lister) inside the retry so the
	// ResourceVersion is fresh; the initial read above came from the informer
	// cache and can be stale, which has caused frequent 409 conflicts under
	// contention with the refresh-consumer writeback.
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := r.userAttributes.Get(userName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			// The user attribute was deleted between the cache read and this
			// retry (user cleanup, provider disable). Nothing to refresh.
			return nil
		}
		if err != nil {
			return err
		}
		if _, ok := latest.Annotations[common.ProviderRefreshErrorAnnotation]; ok {
			return nil
		}
		if latest.NeedsRefresh {
			return nil
		}

		latest.NeedsRefresh = true
		_, err = r.userAttributes.Update(latest)
		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	})
	if err != nil {
		logrus.Errorf("Error updating user attribute to trigger refresh: %v", err)
	}
}

func (r *refresher) deleteToken(token accessor.TokenAccessor) error {
	switch token.(type) {
	case *apiv3.Token:
		return r.tokens.Delete(token.GetName(), &metav1.DeleteOptions{})
	case *ext.Token:
		return r.extTokenStore.Delete(token.GetName(), &metav1.DeleteOptions{})
	default:
		return fmt.Errorf("unable to delete token of unknown type %T", token)
	}
}

func (r *refresher) disableToken(token accessor.TokenAccessor) error {
	switch t := token.(type) {
	case *apiv3.Token:
		v3Token := t.DeepCopy()
		v3Token.Enabled = ptr.To(false)
		_, err := r.tokenMGR.UpdateToken(v3Token)
		return err
	case *ext.Token:
		return r.extTokenStore.Disable(t.GetName())
	default:
		return fmt.Errorf("unable to disable token of unknown type %T", token)
	}
}

func (r *refresher) deleteLoginTokens(providerName string, loginTokens map[string][]accessor.TokenAccessor) error {
	for _, token := range loginTokens[providerName] {
		if err := r.deleteToken(token); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// refreshAttributes re-evaluates a user's access across all configured auth
// providers. For each provider it refreshes group memberships, checks whether
// the user can still log in, and cleans up login tokens for providers that
// rejected the user. If no provider confirms access and none had transient
// errors, all derived (non-login) tokens are disabled. Transient errors are
// treated conservatively: tokens are preserved to avoid locking out users due
// to temporary IdP failures.
func (r *refresher) refreshAttributes(attribs *apiv3.UserAttribute) (*apiv3.UserAttribute, error) {
	var (
		derivedTokenList      []accessor.TokenAccessor
		canLogInAtAll         bool
		errorConfirmingLogins bool
	)

	attribs = attribs.DeepCopy()

	user, err := r.userLister.Get("", attribs.Name)
	if err != nil {
		return nil, fmt.Errorf("error getting user %s: %w", attribs.Name, err)
	}

	loginTokens := make(map[string][]accessor.TokenAccessor)
	derivedTokens := make(map[string][]accessor.TokenAccessor)

	assign := func(token accessor.TokenAccessor) {
		provider := token.GetAuthProvider()
		if token.GetIsDerived() {
			derivedTokens[provider] = append(derivedTokens[provider], token)
			derivedTokenList = append(derivedTokenList, token)
		} else {
			loginTokens[provider] = append(loginTokens[provider], token)
		}
	}

	tokenUserIDLabelSet := labels.Set(map[string]string{tokens.UserIDLabel: user.Name})
	v3Tokens, err := r.tokenLister.List("", tokenUserIDLabelSet.AsSelector())
	if err != nil {
		return nil, fmt.Errorf("error listing tokens for user %s: %w", user.Name, err)
	}

	for _, token := range v3Tokens {
		assign(token)
	}

	extTokens, err := r.extTokenStore.ListForUser(user.Name)
	if err != nil {
		return nil, fmt.Errorf("error listing ext tokens for user %s: %w", user.Name, err)
	}

	for _, token := range extTokens.Items {
		assign(&token)
	}

	for _, providerName := range providers.ProviderNames() {
		canAccess, errConfirming, err := r.refreshProvider(attribs, providerName, user, loginTokens, derivedTokens, errorConfirmingLogins)
		if err != nil {
			return nil, err
		}
		if errConfirming {
			errorConfirmingLogins = true
		}
		if canAccess {
			canLogInAtAll = true
		}
	}

	if canLogInAtAll || errorConfirmingLogins {
		return attribs, nil
	}

	for _, token := range derivedTokenList {
		if err := r.disableToken(token); err != nil {
			return nil, fmt.Errorf("error disabling token %s: %w", token.GetName(), err)
		}
	}

	return attribs, nil
}

// refreshProvider refreshes a single provider's state within attribs.
// It returns whether the user can access the provider and whether there was
// an error confirming their login (which prevents token cleanup).
func (r *refresher) refreshProvider(
	attribs *apiv3.UserAttribute,
	providerName string,
	user *apiv3.User,
	loginTokens, derivedTokens map[string][]accessor.TokenAccessor,
	errorConfirmingLogins bool,
) (canAccess bool, errConfirming bool, err error) {
	principalID := GetPrincipalIDForProvider(providerName, user)

	providerDisabled, err := providers.IsDisabledProvider(providerName)
	if err != nil {
		logrus.Warnf("Unable to determine if provider %s was disabled, will assume that it isn't with error: %v", providerName, err)
		providerDisabled = false
	}
	if providerDisabled {
		principalID = ""
	}

	if principalID == "" {
		attribs.GroupPrincipals[providerName] = apiv3.Principals{}
		if !errorConfirmingLogins {
			if err := r.deleteLoginTokens(providerName, loginTokens); err != nil {
				return false, false, err
			}
		}
		return false, false, nil
	}

	newGroupPrincipals, canRefresh, errConfirming, principalID, err := r.refreshGroupPrincipals(attribs, providerName, user.Name, principalID, loginTokens)
	if err != nil {
		return false, false, err
	}

	if len(newGroupPrincipals) == 0 {
		newGroupPrincipals = nil
	}
	attribs.GroupPrincipals[providerName] = apiv3.Principals{Items: newGroupPrincipals}

	if principalID != "" && !errorConfirmingLogins && !errConfirming {
		canStillAccess, err := providers.CanAccessWithGroupProviders(providerName, principalID, newGroupPrincipals)
		if err != nil {
			return false, false, err
		}
		canAccess = canStillAccess
	}

	if principalID != "" && canRefresh && (len(loginTokens[providerName]) > 0 || (len(derivedTokens[providerName]) > 0 && (canAccess || errorConfirmingLogins || errConfirming))) {
		var token accessor.TokenAccessor
		if len(loginTokens[providerName]) > 0 {
			token = loginTokens[providerName][0]
		} else {
			token = derivedTokens[providerName][0]
		}
		userPrincipal, err := providers.GetPrincipal(principalID, token)
		if err != nil {
			return false, false, err
		}
		userExtraInfo := providers.GetUserExtraAttributes(providerName, userPrincipal)
		if userExtraInfo != nil {
			if attribs.ExtraByProvider == nil {
				attribs.ExtraByProvider = make(map[string]map[string][]string)
			}
			attribs.ExtraByProvider[providerName] = userExtraInfo
		}
	}

	if !canAccess && !errorConfirmingLogins && !errConfirming {
		if err := r.deleteLoginTokens(providerName, loginTokens); err != nil {
			return false, false, err
		}
	}

	return canAccess, errConfirming, nil
}

// refreshGroupPrincipals fetches or preserves group principals for a provider.
func (r *refresher) refreshGroupPrincipals(
	attribs *apiv3.UserAttribute,
	providerName string,
	userName string,
	principalID string,
	loginTokens map[string][]accessor.TokenAccessor,
) (groups []apiv3.Principal, canRefresh bool, errConfirming bool, updatedPrincipalID string, err error) {
	canRefresh = true
	updatedPrincipalID = principalID
	secret := ""

	if providers.ProviderUsesUserSecrets(providerName) {
		secret, err = r.tokenMGR.GetSecret(userName, providerName, loginTokens[providerName])
		if apierrors.IsNotFound(err) {
			canRefresh = false
			errConfirming = true
		} else if err != nil {
			return nil, false, false, principalID, err
		}
	}

	if !providers.ProviderCanRefreshPrincipals(providerName) || !canRefresh {
		existing := attribs.GroupPrincipals[providerName].Items
		if existing != nil {
			groups = existing
		}
		return groups, canRefresh, errConfirming, updatedPrincipalID, nil
	}

	groups, err = providers.RefetchGroupPrincipals(principalID, providerName, secret)
	if err == nil {
		return groups, canRefresh, errConfirming, updatedPrincipalID, nil
	}

	// Non-transient errors must propagate so the controller can annotate
	// the UserAttribute and stop requeuing.
	var nte *common.NonTransientError
	if errors.As(err, &nte) {
		return nil, false, false, principalID, err
	}

	if err.Error() != "no access" {
		errConfirming = true
		logrus.Warnf(
			"Error refreshing token principals for auth provider %s, userattribute %s, principal %s, skipping: %v",
			providerName, attribs.Name, principalID, err,
		)
		existing := attribs.GroupPrincipals[providerName].Items
		if existing != nil {
			groups = existing
		}
		return groups, canRefresh, errConfirming, updatedPrincipalID, nil
	}

	// User explicitly cannot log in (e.g. they no longer exist in the IdP).
	// Blank their principal so login tokens get cleaned up.
	updatedPrincipalID = ""
	return nil, canRefresh, errConfirming, updatedPrincipalID, nil
}

func GetPrincipalIDForProvider(providerName string, user *apiv3.User) string {
	prefix := providerName + "_user://"
	if providerName == "local" {
		prefix = "local://"
	}

	principalID := ""
	for _, id := range user.PrincipalIDs {
		if strings.HasPrefix(id, prefix) {
			principalID = id
			break
		}
	}

	return principalID
}
