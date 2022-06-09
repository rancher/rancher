package management

import (
	"time"

	"github.com/rancher/rancher/pkg/agent/clean"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dupeBindingsCleanupKey          = "DedupeBindingsDone"
	orphanBindingsCleanupKey        = "CleanupOrphanBindingsDone"
	orphanCatalogBindingsCleanupKey = "CleanupOrphanCatalogBindingsDone"
)

func CleanupDuplicateBindings(scaledContext *config.ScaledContext, wContext *wrangler.Context) {
	// check if duplicate binding cleanup has run already
	logrus.Infof("checking configmap %s/%s to determine if duplicate bindings cleanup needs to run", cattleNamespace, bootstrapAdminConfig)
	if adminConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(scaledContext.RunContext, bootstrapAdminConfig, v1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Warnf("unable to determine if duplicate bindings cleanup has ran, skipping: %v", err)
			return
		}
	} else {
		// config map already exists, check if the cleanup key is found
		if _, ok := adminConfig.Data[dupeBindingsCleanupKey]; ok {
			//cleanup has been run already, nothing to do here
			logrus.Info("duplicate bindings cleanup has already run, skipping")
			return
		}
		// run cleanup after delay to give other controllers a chance to create CRTBs/PRTBs and ease the load on the API at startup
		const delayMinutes = 3
		logrus.Infof("bindings cleanup needed, waiting %v minutes before starting", delayMinutes)
		time.Sleep(time.Minute * delayMinutes)
		logrus.Info("starting duplicate binding cleanup")
		err = clean.DuplicateBindings(&scaledContext.RESTConfig)
		if err != nil {
			logrus.Warnf("error in cleaning up duplicate bindings: %v", err)
			return
		}
		// update configmap
		reloadedConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(scaledContext.RunContext, bootstrapAdminConfig, v1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logrus.Warnf("unable to load configmap %v: %v", bootstrapAdminConfig, err)
				return
			}
		}

		adminConfigCopy := reloadedConfig.DeepCopy()
		if adminConfigCopy.Data == nil {
			adminConfigCopy.Data = make(map[string]string)
		}
		adminConfigCopy.Data[dupeBindingsCleanupKey] = "yes"

		_, err = wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Update(scaledContext.RunContext, adminConfigCopy, v1.UpdateOptions{})
		if err != nil {
			logrus.Warnf("error %v in updating %v configmap to record that the duplicate binding cleanup is done", err, bootstrapAdminConfig)
		}
		logrus.Infof("successfully cleaned up duplicate bindings")
	}
}

func CleanupOrphanBindings(scaledContext *config.ScaledContext, wContext *wrangler.Context) {
	err := cleanupSpecificOrphanedBindings(scaledContext, wContext, orphanBindingsCleanupKey)
	if err != nil {
		logrus.Errorf("failed to cleanup orphan bindings")
	}
	err = cleanupSpecificOrphanedBindings(scaledContext, wContext, orphanCatalogBindingsCleanupKey)
	if err != nil {
		logrus.Errorf("failed to cleanup orphan catalog bindings")
	}
}

// Runs the cleanup process for orphaned bindings given a cleanupKey specifying which cleanup job should be run (orphanBindings or orphanCatalogBindings)
func cleanupSpecificOrphanedBindings(scaledContext *config.ScaledContext, wContext *wrangler.Context, cleanupKey string) error {
	logrus.Infof("checking configmap %s/%s to determine if orphan bindings cleanup needs to run", cattleNamespace, bootstrapAdminConfig)
	adminConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(scaledContext.RunContext, bootstrapAdminConfig, v1.GetOptions{})
	if err != nil {
		logrus.Warnf("[%v] unable to determine if bindings cleanup has ran, skipping: %v", cleanupKey, err)
		return err
	}

	// config map exists, check if the cleanup key is found
	if _, ok := adminConfig.Data[cleanupKey]; ok {
		logrus.Infof("[%v] orphan bindings cleanup has already run, skipping", cleanupKey)
		return nil
	}

	// run cleanup
	if cleanupKey == orphanBindingsCleanupKey {
		err = clean.OrphanBindings(&scaledContext.RESTConfig)
	} else {
		err = clean.OrphanCatalogBindings(&scaledContext.RESTConfig)
	}
	if err != nil {
		logrus.Warnf("[%v] error during orphan binding cleanup: %v", cleanupKey, err)
		return err
	}

	// update configmap
	reloadedConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(scaledContext.RunContext, bootstrapAdminConfig, v1.GetOptions{})
	if err != nil {
		logrus.Warnf("[%v] unable to get configmap %v: %v", cleanupKey, bootstrapAdminConfig, err)
		return err
	}

	adminConfigCopy := reloadedConfig.DeepCopy()
	if adminConfigCopy.Data == nil {
		adminConfigCopy.Data = make(map[string]string)
	}
	adminConfigCopy.Data[cleanupKey] = "yes"

	_, err = wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Update(scaledContext.RunContext, adminConfigCopy, v1.UpdateOptions{})
	if err != nil {
		logrus.Warnf("[%v] error %v while updating configmap %v, unable to record completion of orphan binding cleanup", cleanupKey, err, bootstrapAdminConfig)
	}

	logrus.Infof("[%v] successfully cleaned up orphan bindings", cleanupKey)
	return nil
}
