// Package auth contains handlers and helpful functions for managing authentication. This includes token cleanup and
// managing Rancher's RBAC kubernetes resources: ClusterRoleTemplateBindings and ProjectRoleTemplateBindings.
package auth

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	"github.com/rancher/rancher/pkg/auth/userretention"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
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
	userAttributes            mgmtcontrollers.UserAttributeClient
	providerRefresh           func(attribs *v3.UserAttribute) (*v3.UserAttribute, error)
	ensureUserRetentionLabels func(attribs *v3.UserAttribute) error
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
func (c *UserAttributeController) sync(key string, attribs *v3.UserAttribute) (runtime.Object, error) {
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

	attribs, err = c.providerRefresh(attribs)
	if err != nil {
		return nil, fmt.Errorf("error refreshing user attribute %s: %w", name, err)
	}

	updated, err := c.userAttributes.Update(attribs)
	if err == nil {
		return updated, nil
	}

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
	newAttribs.GroupPrincipals = attribs.GroupPrincipals
	newAttribs.ExtraByProvider = attribs.ExtraByProvider

	updated, nerr = c.userAttributes.Update(newAttribs)
	if nerr != nil {
		logrus.Errorf("error updating new version of user attribute %s: %v", name, nerr)
		return nil, err // Deliberately return the original error.
	}

	return updated, nil
}
