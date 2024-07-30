package userretention

import (
	"context"
	"fmt"
	"strconv"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

// UserLabeler sets user retention labels based on user attributes and settings.
type UserLabeler struct {
	ctx                context.Context
	userAttributeCache mgmtcontrollers.UserAttributeCache
	userCache          mgmtcontrollers.UserCache
	users              mgmtcontrollers.UserClient
	readSettings       func() (settings, error)
}

// NewUserLabeler creates a new instance of UserLabeler.
func NewUserLabeler(ctx context.Context, wContext *wrangler.Context) *UserLabeler {
	return &UserLabeler{
		ctx:                ctx,
		userCache:          wContext.Mgmt.User().Cache(),
		users:              wContext.Mgmt.User(),
		userAttributeCache: wContext.Mgmt.UserAttribute().Cache(),
		readSettings:       readSettings,
	}
}

// EnsureForAll sets retention labels for all users.
func (l *UserLabeler) EnsureForAll() error {
	if l.ctx.Err() != nil {
		logrus.Info("userretention: labeler: context canceled, quitting")
		return nil
	}

	settings, err := l.readSettings()
	if err != nil {
		// We don't want the caller (UserAttribute controller) to spin indefinitely.
		// Log the error and return early.
		logrus.Errorf("userretention: labeler: error reading settings, retention is disabled: %v", err)
		return nil
	}

	users, err := l.userCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("userretention: labeler: error listing users: %w", err)
	}

	var processed, skipped int

	for _, user := range users {
		if l.ctx.Err() != nil {
			logrus.Info("userretention: labeler: context canceled, quitting")
			break
		}

		if user.IsSystem() {
			// Note: although the default admin is not subject to retention
			// we still want to check/set the last-login label for it.
			continue
		}

		processed++

		attribs, err := l.userAttributeCache.Get(user.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			logrus.Errorf("userretention: labeler: error getting user attributes for %s: %v", user.Name, err)
			skipped++
			continue
		}

		if attribs == nil {
			// This is possible if the user was created but hasn't logged in yet.
			logrus.Debugf("userretention: labeler: no user attributes found for %s, skipping", user.Name)
			skipped++
			continue
		}

		l.setLabelsAndUpdateUser(settings, user, attribs)
	}

	return nil
}

// EnsureForAttributes sets retention labels for a user based on user attributes.
func (l *UserLabeler) EnsureForAttributes(attribs *mgmtv3.UserAttribute) error {
	if l.ctx.Err() != nil {
		logrus.Info("userretention: labeler: context canceled, quitting")
		return nil
	}

	settings, err := l.readSettings()
	if err != nil {
		// We don't want the caller (Setting controller) to spin indefinitely.
		// Log the error and return early.
		logrus.Errorf("userretention: labeler: error reading settings: %v, retention is disabled", err)
		return nil
	}

	user, err := l.userCache.Get(attribs.Name)
	if err != nil {
		// In a highly unlikely event of having userattribute without a corresponding user object
		// we don't want the caller to spin indefinitely. There is nothing we can do about it,
		// other than to log the error and move on.
		if apierrors.IsNotFound(err) {
			logrus.Errorf("userretention: labeler: error getting user: user not found for user attributes %s", attribs.Name)
			return nil
		}

		return fmt.Errorf("userretention: labeler: error getting user %s: %w", user.Name, err)
	}

	l.setLabelsAndUpdateUser(settings, user, attribs)

	return nil
}

// setLabelsAndUpdateUser sets user retention labels and updates the user.
func (l *UserLabeler) setLabelsAndUpdateUser(settings settings, user *mgmtv3.User, attribs *mgmtv3.UserAttribute) {
	var userGetTry int
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		defer func() { userGetTry++ }()

		var err error
		if userGetTry > 0 { // Refetch only if the first attempt to update failed.
			user, err = l.users.Get(user.Name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) { // The user is no longer, move on.
					return nil
				}

				logrus.Errorf("userretention: labeler: error getting user %s: %v", user.Name, err)
				return err
			}
		}

		updated := setLabels(settings, user, attribs)
		if !updated { // If labels haven't changed, return early.
			return nil
		}

		_, err = l.users.Update(user)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Errorf("userretention: labeler: error updating user: user not found %s", user.Name)
				return nil
			}

			return err
		}

		return nil
	})
	if err != nil {
		// Log the error and move on.
		logrus.Errorf("userretention: labeler: error updating user %s: %v", user.Name, err)
	}
}

// setLabels sets user labels based on user retention settings and user attributes.
func setLabels(settings settings, user *mgmtv3.User, attribs *mgmtv3.UserAttribute) bool {
	if user.Labels == nil {
		user.Labels = map[string]string{}
	}

	lastLogin := lastLoginTime(settings, attribs)
	updated := ensureLabel(lastLogin, LastLoginLabelKey, user)

	var deleteAfterTime, disableAfterTime time.Time

	if settings.ShouldDelete() && !user.IsDefaultAdmin() && !lastLogin.IsZero() {
		if attribs.DeleteAfter != nil {
			if userDeleteAfter := attribs.DeleteAfter.Duration; userDeleteAfter > 0 {
				deleteAfterTime = lastLogin.Add(userDeleteAfter) // User-specific override.
			}
		} else {
			deleteAfterTime = lastLogin.Add(settings.deleteAfter)
		}
	}
	updated = ensureLabel(deleteAfterTime, DeleteAfterLabelKey, user) || updated

	if settings.ShouldDisable() && !user.IsDefaultAdmin() && !lastLogin.IsZero() {
		if attribs.DisableAfter != nil {
			if userDisableAfter := attribs.DisableAfter.Duration; userDisableAfter > 0 {
				disableAfterTime = lastLogin.Add(userDisableAfter) // User-specific override.
			}
		} else {
			disableAfterTime = lastLogin.Add(settings.disableAfter)
		}
	}
	return ensureLabel(disableAfterTime, DisableAfterLabelKey, user) || updated
}

// toEpochTimeString returns the epoch time as a string.
func toEpochTimeString(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

// ensureLabel checks and sets or deletes a user retention label.
// It returns true if the label was changed.
func ensureLabel(value time.Time, labelKey string, user *mgmtv3.User) bool {
	if value.IsZero() {
		if _, ok := user.Labels[labelKey]; ok {
			delete(user.Labels, labelKey)
			return true
		}
		return false
	}

	label := toEpochTimeString(value)
	if user.Labels[labelKey] != label {
		user.Labels[labelKey] = label
		return true
	}

	return false
}
