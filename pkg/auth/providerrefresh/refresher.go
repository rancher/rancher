package providerrefresh

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/settings"
	"github.com/rancher/rancher/pkg/auth/tokens"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
)

type UserAuthRefresher interface {
	TriggerAllUserRefresh()
	TriggerUserRefresh(string, bool)
}

func NewUserAuthRefresher(ctx context.Context, scaledContext *config.ScaledContext) UserAuthRefresher {
	extTokenStore := exttokenstore.NewSystemFromWrangler(scaledContext.Wrangler)

	return &refresher{
		tokenLister:         scaledContext.Management.Tokens("").Controller().Lister(),
		tokens:              scaledContext.Management.Tokens(""),
		userLister:          scaledContext.Management.Users("").Controller().Lister(),
		tokenMGR:            tokens.NewManager(ctx, scaledContext),
		userAttributes:      scaledContext.Management.UserAttributes(""),
		userAttributeLister: scaledContext.Management.UserAttributes("").Controller().Lister(),
		extTokenStore:       extTokenStore,
	}
}

type refresher struct {
	sync.Mutex
	tokenLister         v3.TokenLister
	tokens              v3.TokenInterface
	tokenMGR            *tokens.Manager
	userLister          v3.UserLister
	userAttributes      v3.UserAttributeInterface
	userAttributeLister v3.UserAttributeLister
	intervalInSeconds   int64
	unparsedMaxAge      string
	maxAge              time.Duration
	extTokenStore       *exttokenstore.SystemStore
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
	attribs, needCreate, err := r.tokenMGR.EnsureAndGetUserAttribute(userName)
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

	attribs.NeedsRefresh = true
	if needCreate {
		_, err := r.userAttributes.Create(attribs)
		if err != nil {
			logrus.Errorf("Error creating user attribute to trigger refresh: %v", err)
		}
	} else {
		_, err = r.userAttributes.Update(attribs)
		if err != nil {
			if apierrors.IsConflict(err) {
				// User attribute has just been updated, triggering the refresh.
				logrus.Debugf("Error updating user attribute to trigger refresh: %v", err)
			} else {
				logrus.Errorf("Error updating user attribute to trigger refresh: %v", err)
			}
		}
	}
}

func (r *refresher) refreshAttributes(attribs *v3.UserAttribute) (*v3.UserAttribute, error) {
	var (
		derivedTokenList      []accessor.TokenAccessor
		derivedTokens         map[string][]accessor.TokenAccessor
		loginTokenList        []accessor.TokenAccessor
		loginTokens           map[string][]accessor.TokenAccessor
		canLogInAtAll         bool
		errorConfirmingLogins bool
	)

	attribs = attribs.DeepCopy()

	user, err := r.userLister.Get("", attribs.Name)
	if err != nil {
		return nil, err
	}

	loginTokens = make(map[string][]accessor.TokenAccessor)
	derivedTokens = make(map[string][]accessor.TokenAccessor)

	// List all tokens, norman and ext.
	// For ext tokens we actually filter for the user here.

	allNormanTokens, err := r.tokenLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}

	allExtTokens, err := r.extTokenStore.ListForUser(user.Name)
	if err != nil {
		return nil, err
	}

	// Merge the separate lists into a unified set

	allTokens := []accessor.TokenAccessor{}
	for _, token := range allNormanTokens {
		allTokens = append(allTokens, token)
	}
	for _, eToken := range allExtTokens.Items {
		ex := eToken
		allTokens = append(allTokens, &ex)
	}

	// split into derived versus login tokens, filter for the user

	for providerName := range providers.ProviderNames {
		loginTokens[providerName] = []accessor.TokenAccessor{}
		derivedTokens[providerName] = []accessor.TokenAccessor{}
	}

	for _, token := range allTokens {
		// Needed for the norman tokens. Ext has already filtered for this.
		if token.GetUserID() != user.Name {
			continue
		}

		ap := token.GetAuthProvider()
		if token.GetIsDerived() {
			derivedTokens[ap] = append(derivedTokens[ap], token)
			derivedTokenList = append(derivedTokenList, token)
		} else {
			loginTokens[ap] = append(loginTokens[ap], token)
			loginTokenList = append(loginTokenList, token)
		}
	}

	// per provider ...

	for providerName := range providers.ProviderNames {
		// We have to find out if the user has a userprincipal for the provider.
		principalID := GetPrincipalIDForProvider(providerName, user)
		var newGroupPrincipals []v3.Principal

		providerDisabled, err := providers.IsDisabledProvider(providerName)
		if err != nil {
			logrus.Warnf("Unable to determine if provider %s was disabled, will assume that it isn't with error: %v", providerName, err)
			// this is set as false by the return, but it's re-set here to be explicit/safe about the behavior
			providerDisabled = false
		}
		if providerDisabled {
			// if this auth provider has been disabled, act as though the user lost access to this provider
			principalID = ""
		}

		// If there is no principalID for the provider, there is no reason to go through the refetch process
		if principalID != "" {
			secret := ""
			hasPerUserSecrets, err := providers.ProviderHasPerUserSecrets(providerName)
			if err != nil {
				return nil, err
			}
			if hasPerUserSecrets {
				secret, err = r.tokenMGR.GetSecret(user.Name, providerName, loginTokens[providerName])
				if apierrors.IsNotFound(err) {
					// There is no secret so we can't refresh, just continue to the next attribute
					return attribs, nil
				}
				if err != nil {
					return nil, err
				}
			}

			// SAML cannot refresh, so we do restore the existing providers.
			if providers.UnrefreshableProviders[providerName] {
				existingPrincipals := attribs.GroupPrincipals[providerName].Items
				if existingPrincipals != nil {
					newGroupPrincipals = existingPrincipals
				}
			} else {
				newGroupPrincipals, err = providers.RefetchGroupPrincipals(principalID, providerName, secret)
				if err != nil {
					// In the case that we cant access a server, we still want to continue refreshing, but
					// we no longer want to disable derived tokens, or remove their login tokens for this provider
					if err.Error() != "no access" {
						errorConfirmingLogins = true
						logrus.Warnf("Error refreshing token principals, skipping: %v", err)
						existingPrincipals := attribs.GroupPrincipals[providerName].Items
						if existingPrincipals != nil {
							newGroupPrincipals = existingPrincipals
						}
					} else {
						// In the case that the user explicitly cannot login at all to this provider
						// (e.g. they no longer exist) we pretend they have no principal with this provider
						// so that their login tokens get blanked out
						principalID = ""
					}
				}
			}
		}

		if len(newGroupPrincipals) == 0 {
			newGroupPrincipals = nil
		}

		attribs.GroupPrincipals[providerName] = v32.Principals{Items: newGroupPrincipals}

		canAccessProvider := false

		if principalID != "" && !errorConfirmingLogins {
			// We want to verify that the user still has rancher access
			canStillAccess, err := providers.CanAccessWithGroupProviders(providerName, principalID, newGroupPrincipals)
			if err != nil {
				return nil, err
			}

			if canStillAccess {
				canAccessProvider = true
				canLogInAtAll = true
			}
		}

		// Update extras if either the user has an active login token, or an API token/kubeconfig token and is still active in the auth provider.
		// If the user cannot access the auth provider, the derived tokens are deactivated below and should not be used to determine extra attributes.
		if principalID != "" && (len(loginTokens[providerName]) > 0 || (len(derivedTokens[providerName]) > 0 && (canAccessProvider || errorConfirmingLogins))) {
			// A user is 1:1 with its principal for a given provider, no need to get principals from tokens beyond the first one
			var token accessor.TokenAccessor
			if len(loginTokens[providerName]) > 0 {
				token = loginTokens[providerName][0]
			} else {
				token = derivedTokens[providerName][0]
			}
			userPrincipal, err := providers.GetPrincipal(principalID, token)
			if err != nil {
				return nil, err
			}
			userExtraInfo := providers.GetUserExtraAttributes(providerName, userPrincipal)
			if userExtraInfo != nil {
				if attribs.ExtraByProvider == nil {
					attribs.ExtraByProvider = make(map[string]map[string][]string)
				}
				attribs.ExtraByProvider[providerName] = userExtraInfo
			}
		}

		// If the user doesn't have access through this provider, we want to remove their
		// login tokens for this provider
		if !canAccessProvider && !errorConfirmingLogins {
			for _, token := range loginTokens[providerName] {
				// Deletion is type-dependent
				var err error
				switch token.(type) {
				case *v3.Token:
					err = r.tokens.Delete(token.GetName(), &metav1.DeleteOptions{})
				case *ext.Token:
					err = r.extTokenStore.Delete(token.GetName(), &metav1.DeleteOptions{})
				default:
					err = fmt.Errorf("Unable to delete token of unknown type %T", token)
				}
				if err != nil {
					if apierrors.IsNotFound(err) {
						continue
					}
					return nil, err
				}
			}
		}
	}

	// if they can still log in, or we failed to validate one of their logins, don't disable derived tokens
	if canLogInAtAll || errorConfirmingLogins {
		return attribs, nil
	}

	// user has been deactivated, disable their tokens
	for _, token := range derivedTokenList {
		// Update is type-dependent
		var err error
		switch token.(type) {
		case *v3.Token:
			normanToken := token.(*v3.Token).DeepCopy()
			normanToken.Enabled = pointer.Bool(false)
			_, err = r.tokenMGR.UpdateToken(normanToken)
		case *ext.Token:
			extToken := token.(*ext.Token).DeepCopy()
			extToken.Spec.Enabled = pointer.Bool(false)
			r.extTokenStore.Update(extToken, &metav1.UpdateOptions{})
		default:
			err = fmt.Errorf("Unable to update token of unknown type %T", token)
		}
		if err != nil {
			return nil, err
		}
	}
	return attribs, err
}

func GetPrincipalIDForProvider(providerName string, user *v3.User) string {
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
