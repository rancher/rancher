// Package auth contains handlers and helpful functions for managing authentication. This includes token cleanup and
// managing Rancher's RBAC kubernetes resources: ClusterRoleTemplateBindings and ProjectRoleTemplateBindings.
package auth

import (
	"fmt"
	"strconv"

	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	userAttributeController = "mgmt-auth-userattributes-controller"
)

type UserAttributeController struct {
	userAttributes  v3.UserAttributeInterface
	userLister      v3.UserLister
	users           v3.UserInterface
	providerRefresh func(attribs *v3.UserAttribute) (*v3.UserAttribute, error)
}

func newUserAttributeController(mgmt *config.ManagementContext) *UserAttributeController {
	return &UserAttributeController{
		userAttributes:  mgmt.Management.UserAttributes(""),
		userLister:      mgmt.Management.Users("").Controller().Lister(),
		users:           mgmt.Management.Users(""),
		providerRefresh: providerrefresh.RefreshAttributes,
	}
}

// sync is called periodically and on real updates
func (ua *UserAttributeController) sync(key string, obj *v3.UserAttribute) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return nil, nil
	}

	err := ua.ensureLastLoginLabel(obj)
	if err != nil {
		return nil, fmt.Errorf("error ensuring last-login label for user %s: %w", obj.Name, err)
	}

	if !obj.NeedsRefresh {
		return obj, nil
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
	obj, err = ua.userAttributes.Get(obj.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting user attribute %s before provider refresh: %w", obj.Name, err)
	}
	if !obj.NeedsRefresh {
		return obj, nil
	}

	obj, err = ua.providerRefresh(obj)
	if err != nil {
		return nil, fmt.Errorf("error refreshing user attribute %s: %w", obj.Name, err)
	}

	updated, err := ua.userAttributes.Update(obj)
	if err == nil {
		return updated, nil
	}

	err = fmt.Errorf("error updating user attribute %s after provider refresh: %w", obj.Name, err)
	if !apierrors.IsConflict(err) {
		return nil, err
	}

	nobj, nerr := ua.userAttributes.Get(obj.Name, metav1.GetOptions{})
	if nerr != nil {
		logrus.Errorf("error getting new version of user attribute %s: %v", obj.Name, nerr)
		return nil, err // Deliberately return the original error.
	}

	nobj.NeedsRefresh = obj.NeedsRefresh
	nobj.LastRefresh = obj.LastRefresh
	nobj.GroupPrincipals = obj.GroupPrincipals
	nobj.ExtraByProvider = obj.ExtraByProvider

	updated, nerr = ua.userAttributes.Update(nobj)
	if nerr != nil {
		logrus.Errorf("error updating new version of user attribute %s: %v", obj.Name, nerr)
		return nil, err // Deliberately return the original error.
	}

	return updated, nil
}

const labelLastLoginKey = "cattle.io/last-login"

func (ua *UserAttributeController) ensureLastLoginLabel(attribs *v3.UserAttribute) error {
	if attribs.LastLogin.IsZero() {
		return nil
	}

	user, err := ua.userLister.Get("", attribs.Name)
	if err != nil {
		// In a highly unlikely event of having userattribute without a corresponding user object
		// we don't want to spin indefinitely. There is nothing we can do about it,
		// other than to log the error and move on.
		if apierrors.IsNotFound(err) {
			logrus.Errorf("error getting user: user not found for the corresponding user attribute %s", attribs.Name)
			return nil
		}

		return fmt.Errorf("error getting user %s: %w", user.Name, err)
	}

	// Calculate the label and retun early if it remains the same.
	lastLoginLabel := strconv.FormatInt(attribs.LastLogin.Time.Unix(), 10)
	if user.Labels[labelLastLoginKey] == lastLoginLabel {
		return nil
	}

	// Do the update only if the label changed.
	if user.Labels == nil {
		user.Labels = map[string]string{}
	}
	user.Labels[labelLastLoginKey] = lastLoginLabel

	_, err = ua.users.Update(user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logrus.Errorf("error updating user: user not found for the corresponding user attribute %s", attribs.Name)
			return nil
		}

		return fmt.Errorf("error updating user %s: %w", user.Name, err)
	}

	return nil
}
