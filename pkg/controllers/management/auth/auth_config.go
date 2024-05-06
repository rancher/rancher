package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rancher/norman/objectclient"
	"github.com/rancher/rancher/pkg/auth/cleanup"
	"github.com/rancher/rancher/pkg/auth/providerrefresh"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	authConfigControllerName = "mgmt-auth-config-controller"

	// CleanupAnnotation exists to prevent admins from running the cleanup routine in two scenarios:
	// 1. When the provider has not been enabled or deliberately disabled, and thus does not need cleanup.
	// 2. When the value of the annotation is 'user-locked', set manually by admins in advance.
	// Rancher will run cleanup only if the provider becomes disabled,
	// and the annotation's value is 'unlocked'.
	CleanupAnnotation = "management.cattle.io/auth-provider-cleanup"

	CleanupUnlocked      = "unlocked"
	CleanupUserLocked    = "user-locked"
	CleanupRancherLocked = "rancher-locked"
)

// CleanupService performs a cleanup of auxiliary resources belonging to a particular auth provider type.
type CleanupService interface {
	Run(config *v3.AuthConfig) error
}

type authConfigController struct {
	users         v3.UserLister
	authRefresher providerrefresh.UserAuthRefresher
	cleanup       CleanupService
	// Note the use of the GenericClient here. AuthConfigs contain internal-only fields that deal with
	// various auth providers. Those fields are not present everywhere, nor are they defined in the CRD. Given
	// that, the regular client will "eat" those internal-only fields, so in this case, we use
	// the unstructured client, losing some validation, but gaining the flexibility we require.
	authConfigsUnstructured objectclient.GenericClient
}

func newAuthConfigController(context context.Context, mgmt *config.ManagementContext, scaledContext *config.ScaledContext) *authConfigController {
	controller := &authConfigController{
		users:                   mgmt.Management.Users("").Controller().Lister(),
		authRefresher:           providerrefresh.NewUserAuthRefresher(context, scaledContext),
		cleanup:                 cleanup.NewCleanupService(mgmt.Core.Secrets(""), mgmt.Wrangler.Mgmt),
		authConfigsUnstructured: scaledContext.Management.AuthConfigs("").ObjectClient().UnstructuredClient(),
	}
	return controller
}

func (ac *authConfigController) getUnstructured(obj *v3.AuthConfig) (*unstructured.Unstructured, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot get a nil auth config")
	}
	runtimeObj, err := ac.authConfigsUnstructured.Get(obj.Name, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get auth config %s from Kubernetes: %w", obj.Name, err)
	}
	unstructuredObj, ok := runtimeObj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("auth config %s is not an unstructured value", obj.Name)
	}
	return unstructuredObj, nil
}

func (ac *authConfigController) setCleanupAnnotation(unstructuredObj *unstructured.Unstructured, value string) {
	annotations := unstructuredObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[CleanupAnnotation] = value
	unstructuredObj.SetAnnotations(annotations)
}

func (ac *authConfigController) sync(key string, obj *v3.AuthConfig) (runtime.Object, error) {
	// If obj is nil, the auth config has been deleted. Rancher currently does not handle deletions gracefully,
	// meaning it does not perform resource cleanup. Admins should disable an auth provider instead of deleting its auth config.
	if obj == nil {
		return nil, nil
	}
	err := ac.refreshUsers(obj)
	if err != nil {
		return obj, err
	}

	unstructuredObj, err := ac.getUnstructured(obj)
	if err != nil {
		return nil, err
	}

	value := obj.Annotations[CleanupAnnotation]
	if value == "" {
		if obj.Enabled {
			value = CleanupUnlocked
		} else {
			value = CleanupRancherLocked
		}
		ac.setCleanupAnnotation(unstructuredObj, value)
		return ac.updateAuthConfig(unstructuredObj, obj)
	}

	if obj.Enabled && value == CleanupRancherLocked {
		ac.setCleanupAnnotation(unstructuredObj, CleanupUnlocked)
		return ac.updateAuthConfig(unstructuredObj, obj)
	}

	if !obj.Enabled {
		refusalFmt := "Refusing to reset the config and clean up resources of the auth provider %s because its auth config annotation %s is set to %s."

		switch value {
		case CleanupUnlocked:
			// First, reset the auth config by removing all but essential metadata fields.
			cfg := unstructuredObj.UnstructuredContent()
			resetAuthConfig(cfg)
			unstructuredObj.SetUnstructuredContent(cfg)

			// Second, run resource cleanup.
			err = ac.cleanup.Run(obj)
			if err != nil {
				return obj, err
			}

			// Third, lock the config after cleanup and commit any updates to it.
			logrus.Infof("The resources of the auth provider %s have been cleaned up successfully, and the auth config fields have been reset. Locking down the cleanup operation.", obj.Name)
			ac.setCleanupAnnotation(unstructuredObj, CleanupRancherLocked)
			return ac.updateAuthConfig(unstructuredObj, obj)
		case CleanupRancherLocked:
			logrus.Infof(refusalFmt, obj.Name, CleanupAnnotation, CleanupRancherLocked)
			return obj, nil
		case CleanupUserLocked:
			logrus.Infof(refusalFmt, obj.Name, CleanupAnnotation, CleanupUserLocked)
			return obj, nil
		default:
			logrus.Infof("Refusing to clean up auth provider %s because its auth config annotation %s is invalid", obj.Name, CleanupAnnotation)
			return obj, nil
		}
	}

	return obj, nil
}

func (ac *authConfigController) updateAuthConfig(unstructuredObj *unstructured.Unstructured, obj *v3.AuthConfig) (*v3.AuthConfig, error) {
	uobj, err := ac.authConfigsUnstructured.Update(obj.Name, unstructuredObj)
	if err != nil {
		return nil, fmt.Errorf("failed to update AuthConfig object: %w", err)
	}
	// We need to return an AuthConfig, but Update deals in terms of unstructured objects.
	// Given that, we need to convert the unstructured object to an AuthConfig.
	// Normally, we'd like to use mapstructure.Decode, but its handling of embedded structs
	// does not give us the desired result in this instance, hence the use of json.
	unObject, ok := uobj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to read to unstructured data")
	}
	data, err := json.Marshal(unObject.UnstructuredContent())
	if err != nil {
		return nil, fmt.Errorf("unable to marshal unstructured object: %w", err)
	}
	result := &v3.AuthConfig{}
	if err := json.Unmarshal(data, result); err != nil {
		return nil, fmt.Errorf("uanble to unmarshal to AuthConfig object: %w", err)
	}
	return result, nil
}

func (ac *authConfigController) refreshUsers(obj *v3.AuthConfig) error {
	// if we have changed an auth config, refresh all users belonging to the auth config. This addresses:
	// Disabling an auth provider - now we disable user access
	// Removing a user from auth provider access - now we will immediately revoke access
	users, err := ac.users.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, user := range users {
		principalID := providerrefresh.GetPrincipalIDForProvider(obj.Name, user)
		if principalID != "" {
			// if we have a principal on this provider, then we need to be refreshed to potentially invalidate
			// access derived from this provider
			ac.authRefresher.TriggerUserRefresh(user.Name, true)
		}
	}
	return nil
}

// resetAuthConfig takes an Auth Config as a map and deletes all entries except those with basic metadata fields.
func resetAuthConfig(cfg map[string]any) {
	retainFields := map[string]bool{"apiVersion": true, "kind": true, "metadata": true, "type": true}
	for field := range cfg {
		if !retainFields[field] {
			delete(cfg, field)
		}
	}
}
