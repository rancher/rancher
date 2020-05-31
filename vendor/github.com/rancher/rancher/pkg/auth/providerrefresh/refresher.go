package providerrefresh

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rancher/rancher/pkg/auth/providers"
	"github.com/rancher/rancher/pkg/auth/settings"
	"github.com/rancher/rancher/pkg/auth/tokens"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	specialFalse = false
	falsePointer = &specialFalse
)

type UserAuthRefresher interface {
	TriggerAllUserRefresh()
	TriggerUserRefresh(string, bool)
}

func NewUserAuthRefresher(ctx context.Context, scaledContext *config.ScaledContext) UserAuthRefresher {
	return &refresher{
		tokenLister:         scaledContext.Management.Tokens("").Controller().Lister(),
		tokens:              scaledContext.Management.Tokens(""),
		userLister:          scaledContext.Management.Users("").Controller().Lister(),
		tokenMGR:            tokens.NewManager(ctx, scaledContext),
		userAttributes:      scaledContext.Management.UserAttributes(""),
		userAttributeLister: scaledContext.Management.UserAttributes("").Controller().Lister(),
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
		derivedTokens         []*v3.Token
		loginTokenList        []*v3.Token
		loginTokens           map[string][]*v3.Token
		canLogInAtAll         bool
		errorConfirmingLogins bool
	)

	attribs = attribs.DeepCopy()

	user, err := r.userLister.Get("", attribs.Name)
	if err != nil {
		return nil, err
	}

	loginTokens = make(map[string][]*v3.Token)

	allTokens, err := r.tokenLister.List("", labels.Everything())
	if err != nil {
		return nil, err
	}

	for providerName := range providers.ProviderNames {
		loginTokens[providerName] = []*v3.Token{}
	}

	for _, token := range allTokens {
		if token.UserID != user.Name {
			continue
		}

		if token.IsDerived {
			derivedTokens = append(derivedTokens, token)
		} else {
			loginTokens[token.AuthProvider] = append(loginTokens[token.AuthProvider], token)
			loginTokenList = append(loginTokenList, token)
		}
	}

	for providerName := range providers.ProviderNames {
		// We have to find out if the user has a userprincipal for the provider.
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
		newGroupPrincipals := []v3.Principal{}

		// If there is no principalID for the provider, there is no reason to go through the refetch process
		if principalID != "" {
			secret := ""
			if providers.ProvidersWithSecrets[providerName] {
				secret, err = r.tokenMGR.GetSecret(user.Name, providerName, loginTokens[providerName])
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
						logrus.Errorf("Error refreshing token principals, skipping: %v", err)
						existingPrincipals := attribs.GroupPrincipals[providerName].Items
						if existingPrincipals != nil {
							newGroupPrincipals = existingPrincipals
						}
						continue
					}

					// In the case that the user explicitly cannot login at all to this provider
					// (e.g. they no longer exist) we pretend they have no principal with this provider
					// so that their login tokens get blanked out
					principalID = ""

				}
			}
		}

		if len(newGroupPrincipals) == 0 {
			newGroupPrincipals = nil
		}

		attribs.GroupPrincipals[providerName] = v3.Principals{Items: newGroupPrincipals}

		canAccessProvider := false

		if principalID != "" {
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

		// If the user doesn't have access through this provider, we want to remove their login tokens for this provider
		if !canAccessProvider {
			for _, token := range loginTokens[providerName] {
				err := r.tokens.Delete(token.Name, &metav1.DeleteOptions{})
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

	for _, token := range derivedTokens {
		token.Enabled = falsePointer
		_, err := r.tokenMGR.UpdateToken(token)
		if err != nil {
			return nil, err
		}
	}

	return attribs, nil
}
