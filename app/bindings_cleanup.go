package app

import (
	"github.com/rancher/rancher/pkg/agent/clean"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	orphanBindingsCleanupKey = "CleanupOrphanBindingsDone"
)

func cleanupOrphanBindings(scaledContext *config.ScaledContext, wContext *wrangler.Context) {
	logrus.Infof("checking configmap %s/%s to determine if orphan bindings cleanup needs to run", cattleNamespace, bootstrapAdminConfig)
	adminConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(scaledContext.RunContext, bootstrapAdminConfig, v1.GetOptions{})
	if err != nil {
		logrus.Warnf("unable to determine if orphan bindings cleanup has ran, skipping: %v", err)
		return
	}

	// config map exists, check if the cleanup key is found
	if _, ok := adminConfig.Data[orphanBindingsCleanupKey]; ok {
		logrus.Info("orphan bindings cleanup has already run, skipping")
		return
	}

	// run cleanup
	err = clean.OrphanBindings(&scaledContext.RESTConfig)
	if err != nil {
		logrus.Warnf("error during orphan binding cleanup: %v", err)
		return
	}

	// update configmap
	reloadedConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(scaledContext.RunContext, bootstrapAdminConfig, v1.GetOptions{})
	if err != nil {
		logrus.Warnf("unable to get configmap %v: %v", bootstrapAdminConfig, err)
		return
	}

	adminConfigCopy := reloadedConfig.DeepCopy()
	if adminConfigCopy.Data == nil {
		adminConfigCopy.Data = make(map[string]string)
	}
	adminConfigCopy.Data[orphanBindingsCleanupKey] = "yes"

	_, err = wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Update(scaledContext.RunContext, adminConfigCopy, v1.UpdateOptions{})
	if err != nil {
		logrus.Warnf("error %v while updating configmap %v, unable to record completion of orphan binding cleanup", err, bootstrapAdminConfig)
	}

	logrus.Infof("successfully cleaned up orphan bindings")
}
