/*
Package controllers contains logic for handlers that belong to controllers and options for configuring controllers.
*/
package controllers

import (
	"os"
	"strings"

	"github.com/rancher/lasso/pkg/cache"
	"github.com/rancher/lasso/pkg/controller"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type controllerContextType string

const (
	User       controllerContextType = "user"
	Scaled     controllerContextType = "scaled"
	Management controllerContextType = "mgmt"
)

// GetOptsFromEnv configures a SharedControllersFactoryOptions using env var and return a pointer to it.
func GetOptsFromEnv(contextType controllerContextType) *controller.SharedControllerFactoryOptions {
	return &controller.SharedControllerFactoryOptions{
		SyncOnlyChangedObjects: syncOnlyChangedObjects(contextType),
		CacheOptions: &cache.SharedCacheFactoryOptions{
			DefaultTweakList: defaultTweakListOptions(),
		},
	}
}

// syncOnlyChangedObjects returns whether the env var CATTLE_SYNC_ONLY_CHANGED_OBJECTS indicates that controllers for the
// given context type should skip running enqueue if the event triggering the update func is not actual update.
func syncOnlyChangedObjects(option controllerContextType) bool {
	skipUpdate := os.Getenv("CATTLE_SYNC_ONLY_CHANGED_OBJECTS")
	if skipUpdate == "" {
		return false
	}
	parts := strings.Split(skipUpdate, ",")

	for _, part := range parts {
		if controllerContextType(part) == option {
			return true
		}
	}
	return false
}

func defaultTweakListOptions() cache.TweakListOptionsFunc {
	globalLabelSelector := os.Getenv("CATTLE_GLOBAL_LABEL_SELECTOR")
	if globalLabelSelector == "" {
		return func(*v1.ListOptions) {}
	}
	return func(opts *v1.ListOptions) {
		opts.LabelSelector = globalLabelSelector
	}
}
