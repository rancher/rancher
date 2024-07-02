package userretention

import (
	"context"
	"fmt"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"
)

const (
	LastLoginLabelKey    = "cattle.io/last-login"
	DisableAfterLabelKey = "cattle.io/disable-after"
	DeleteAfterLabelKey  = "cattle.io/delete-after"
)

// Retention is the user retention process that disables or deletes inactive users
// based on the last time the user was seen (logged in) that is stored as a user attribute.
// Note: Disabling and deleting are independent of each other and are driven by the
// corresponding settings disableAfter and deleteAfter.
// retention can be configured to either
// - do nothing, which is the default (disableAfter == deleteAfter == 0)
// - only disable users (disableAfter > 0 && deleteAfter == 0)
// - progressively disable and delete users (0 < disableAfter < deleteAfter)
// - only delete users (disableAfter == 0 && deleteAfter > 0 or 0 < deleteAfter < disableAfter)
type Retention struct {
	userAttributeCache mgmtcontrollers.UserAttributeCache
	userCache          mgmtcontrollers.UserCache
	users              mgmtcontrollers.UserClient
	readSettings       func() (settings, error)
}

// New creates a new instance of Retention.
func New(wContext *wrangler.Context) *Retention {
	return &Retention{
		userCache:          wContext.Mgmt.User().Cache(),
		users:              wContext.Mgmt.User(),
		userAttributeCache: wContext.Mgmt.UserAttribute().Cache(),
		readSettings:       readSettings,
	}
}

// Run the user retention process.
func (r *Retention) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		logrus.Info("userretention: context canceled, quitting")
		return nil
	}

	startedAt := time.Now()

	settings, err := r.readSettings()
	if err != nil {
		return fmt.Errorf("error reading settings: %w, retention is disabled", err)
	}

	if !settings.ShouldDisable() && !settings.ShouldDelete() {
		logrus.Info("userretention: nothing to do, neither DisableInactiveUserAfter nor DeleteInactiveUserAfter is set")
		return nil
	}

	logrus.Infof(
		"userretention: started (disable-inactive-user-after %s, delete-inactive-user-after %s, user-last-login-default %s, user-retention-dry-run %t)",
		settings.disableAfter, settings.deleteAfter, settings.FormatDefaultLastLogin(), settings.dryRun,
	)

	users, err := r.userCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing users: %w", err)
	}

	var processed, skipped, disabled, deleted, errCount int
	now := time.Now()

	defer func() {
		logrus.Infof(
			"userretention: finished in %v seconds (processed %d, skipped %d, disabled %d, deleted %d, errors %d)",
			time.Since(startedAt).Seconds(),
			processed, skipped, disabled, deleted, errCount,
		)
	}()

	for _, user := range users {
		if ctx.Err() != nil {
			logrus.Info("userretention: context canceled, quitting")
			break
		}

		if !isSubjectToRetention(user) {
			continue
		}

		processed++

		logrus.Debugf("userretention: processing user %s", user.Name)

		attribs, err := r.userAttributeCache.Get(user.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			logrus.Errorf("userretention: error getting user attributes for %s: %v", user.Name, err)
			errCount++
			skipped++
			continue
		}

		if attribs == nil {
			// This is possible if the user was created but haven't logged in yet.
			logrus.Debugf("userretention: no user attributes found for %s, skipping", user.Name)
			skipped++
			continue
		}

		var (
			userDeleteAfter, userDisableAfter time.Duration
			disableUser                       bool
		)

		lastLogin := lastLoginTime(settings, attribs)
		if !lastLogin.IsZero() {
			deleteAfterTime := lastLogin.Add(settings.deleteAfter)
			if attribs.DeleteAfter != nil { // Apply user-specific override.
				if userDeleteAfter = attribs.DeleteAfter.Duration; userDeleteAfter <= 0 {
					deleteAfterTime = time.Time{} // The user shouldn't be considered for deletion.
				} else {
					deleteAfterTime = lastLogin.Add(userDeleteAfter)
				}
			}
			deleteAfterTime = deleteAfterTime.Truncate(time.Second)

			disableAfterTime := lastLogin.Add(settings.disableAfter)
			if attribs.DisableAfter != nil { // Apply user-specific override.
				if userDisableAfter = attribs.DisableAfter.Duration; userDisableAfter <= 0 {
					disableAfterTime = time.Time{} // The user shouldn't be considered for being disabled.
				} else {
					disableAfterTime = lastLogin.Add(userDisableAfter)
				}
			}
			disableAfterTime = disableAfterTime.Truncate(time.Second)

			if attribs.DeleteAfter != nil && userDeleteAfter == 0 &&
				attribs.DisableAfter != nil && userDisableAfter == 0 {
				skipped++ // This is to keep the counter updated.
			}

			if settings.ShouldDelete() && !deleteAfterTime.IsZero() &&
				now.After(deleteAfterTime) {
				logrus.Infof("userretention: deleting user %s", user.Name)

				if !settings.dryRun {
					err := r.users.Delete(user.Name, &metav1.DeleteOptions{})
					if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
						logrus.Errorf("userretention: error deleting user %s: %v", user.Name, err)
						errCount++
						continue
					}
				}

				deleted++
				continue

			}

			if settings.ShouldDisable() && !disableAfterTime.IsZero() &&
				now.After(disableAfterTime) && pointer.BoolDeref(user.Enabled, true) {
				logrus.Infof("userretention: disabling user %s", user.Name)
				// Flag the needed update but don't apply it as we may need to update retention labels too.
				disableUser = true
				disabled++
			}
		}

		var userGetTry int
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			defer func() { userGetTry++ }()

			var err error

			if userGetTry > 0 { // Refetch only if the first attempt to update failed.
				user, err = r.users.Get(user.Name, metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) { // The user is no longer, move on.
						return nil
					}

					logrus.Errorf("userretention: error getting user %s: %v", user.Name, err)
					return err
				}
			}

			if settings.dryRun {
				return nil
			}

			if disableUser {
				user.Enabled = pointer.Bool(false)
			}

			// Update the retention labels if necessary.
			labelsUpdated := setLabels(settings, user, attribs)

			// No user updates; return early.
			if !labelsUpdated && !disableUser {
				return nil
			}

			if _, err = r.users.Update(user); err != nil {
				if apierrors.IsNotFound(err) {
					logrus.Errorf("userretention: error updating user: user not found %s", user.Name)
					return nil
				}

				return err
			}

			return nil
		})
		if err != nil {
			// Log the error and move on.
			logrus.Errorf("userretention: error updating user %s: %v", user.Name, err)
			errCount++
		}
	}

	return nil
}

func isSubjectToRetention(user *v3.User) bool {
	return !user.IsDefaultAdmin() && !user.IsSystem()
}

func lastLoginTime(settings settings, attribs *v3.UserAttribute) time.Time {
	if attribs.LastLogin != nil && !attribs.LastLogin.Time.IsZero() {
		return attribs.LastLogin.Time
	}

	if !settings.defaultLastLogin.IsZero() {
		return settings.defaultLastLogin
	}

	return time.Time{}
}
