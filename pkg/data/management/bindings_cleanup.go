package management

import (
	"context"

	"github.com/rancher/rancher/pkg/agent/clean"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CleanupDuplicateBindings(scaledContext *config.ScaledContext, wContext *wrangler.Context) {
	//check if duplicate binding cleanup has run already
	logrus.Info("CleanupDuplicateBindings, checking configmap")
	if adminConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(context.TODO(), bootstrapAdminConfig, v1.GetOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Warnf("Unable to determine if bindings cleanup already ran or not, skipping run: %v", err)
			return
		}
	} else {
		// config map already exists, check if the cleanup key is found
		if _, ok := adminConfig.Data["DedupeBindingsDone"]; ok {
			//cleanup has been run already, nothing to do here
			logrus.Info("bindings cleanup already ran before, not calling again")
			return
		}
		// run cleanup
		logrus.Info("Calling Duplicate CRB and RB cleanup")
		err = clean.Bindings(&scaledContext.RESTConfig)
		if err != nil {
			logrus.Warnf("Error in cleaning up Duplicate CRB and RB: %v", err)
			return
		}
		//update configmap
		reloadedConfig, err := wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Get(context.TODO(), bootstrapAdminConfig, v1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logrus.Warnf("Unable to load configmap %v: %v", bootstrapAdminConfig, err)
				return
			}
		}
		adminConfigCopy := reloadedConfig.DeepCopy()
		data := make(map[string]string)
		data["DedupeBindingsDone"] = "yes"
		adminConfigCopy.Data = data

		_, err = wContext.K8s.CoreV1().ConfigMaps(cattleNamespace).Update(context.TODO(), adminConfigCopy, v1.UpdateOptions{})
		if err != nil {
			logrus.Warnf("Error %v in updating %v configmap to record that the cleanup of duplicate CRB and RB is done", err, bootstrapAdminConfig)
		}
		logrus.Infof("Done Duplicate CRB and RB cleanup")
	}
}
