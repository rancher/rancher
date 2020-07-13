package watcher

import (
	"context"
	"reflect"
	"time"

	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/rancher/pkg/ticker"
	mgmtv3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type endpointWatcher struct {
	dialerFactory   dialer.Factory
	clusterName     string
	clusterLoggings mgmtv3.ClusterLoggingInterface
	projectLoggings mgmtv3.ProjectLoggingInterface
}

func StartEndpointWatcher(ctx context.Context, cluster *config.UserContext) {
	s := &endpointWatcher{
		dialerFactory:   cluster.Management.Dialer,
		clusterName:     cluster.ClusterName,
		clusterLoggings: cluster.Management.Management.ClusterLoggings(cluster.ClusterName),
		projectLoggings: cluster.Management.Management.ProjectLoggings(metav1.NamespaceAll),
	}
	go s.watch(ctx, 120*time.Second)
}

func (e *endpointWatcher) watch(ctx context.Context, interval time.Duration) {
	for range ticker.Context(ctx, interval) {
		if err := e.checkClusterTarget(ctx); err != nil {
			logrus.Error(err)
		}

		if err := e.checkProjectTarget(ctx); err != nil {
			logrus.Error(err)
		}
	}
}

func (e *endpointWatcher) checkClusterTarget(ctx context.Context) error {
	cls, err := e.clusterLoggings.Controller().Lister().List(e.clusterName, labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "list clusterlogging fail in endpoint watcher")
	}
	if len(cls) == 0 {
		return nil
	}
	obj := cls[0]

	clusterDialer, err := e.dialerFactory.ClusterDialer(obj.Spec.ClusterName)
	if err != nil {
		return errors.Wrapf(err, "get cluster dailer %s failed", obj.Spec.ClusterName)
	}

	wl := utils.NewLoggingTargetTestWrap(obj.Spec.LoggingTargets)
	if wl == nil {
		err = nil
	} else {
		err = wl.TestReachable(ctx, clusterDialer, false)
	}

	updatedObj := setClusterLoggingErrMsg(obj, err)
	if reflect.DeepEqual(updatedObj, obj) {
		return nil
	}
	_, updateErr := e.clusterLoggings.Update(updatedObj)
	if updateErr != errors.Wrapf(updateErr, "set clusterlogging fail in watch endpoint") {
		return updateErr
	}

	return nil
}

func (e *endpointWatcher) checkProjectTarget(ctx context.Context) error {
	clusterDialer, err := e.dialerFactory.ClusterDialer(e.clusterName)
	if err != nil {
		return errors.Wrapf(err, "get cluster dailer %s failed", e.clusterName)
	}

	pls, err := e.projectLoggings.Controller().Lister().List(metav1.NamespaceAll, labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "list clusterlogging fail in endpoint watcher")
	}

	for _, v := range pls {
		wp := utils.NewLoggingTargetTestWrap(v.Spec.LoggingTargets)
		if wp == nil {
			err = nil
		} else {
			err = wp.TestReachable(ctx, clusterDialer, false)
		}

		updatedObj := setProjectLoggingErrMsg(v, err)
		if reflect.DeepEqual(updatedObj, v) {
			continue
		}

		_, updateErr := e.projectLoggings.Update(updatedObj)
		if updateErr != errors.Wrapf(updateErr, "set project fail in watch endpoint") {
			return updateErr
		}
	}

	return nil
}

func setProjectLoggingErrMsg(obj *mgmtv3.ProjectLogging, err error) *mgmtv3.ProjectLogging {
	updatedObj := obj.DeepCopy()
	if err != nil {
		mgmtv3.LoggingConditionUpdated.False(updatedObj)
		mgmtv3.LoggingConditionUpdated.Message(updatedObj, err.Error())
		return updatedObj
	}

	mgmtv3.LoggingConditionUpdated.True(updatedObj)
	mgmtv3.LoggingConditionUpdated.Message(updatedObj, "")
	return updatedObj
}

func setClusterLoggingErrMsg(obj *mgmtv3.ClusterLogging, err error) *mgmtv3.ClusterLogging {
	updatedObj := obj.DeepCopy()
	if err != nil {
		updatedObj.Status.FailedSpec = &obj.Spec
		mgmtv3.LoggingConditionUpdated.False(updatedObj)
		mgmtv3.LoggingConditionUpdated.Message(updatedObj, err.Error())
		return updatedObj
	}

	updatedObj.Status.FailedSpec = nil
	updatedObj.Status.AppliedSpec = obj.Spec

	mgmtv3.LoggingConditionUpdated.True(updatedObj)
	mgmtv3.LoggingConditionUpdated.Message(updatedObj, "")
	return updatedObj
}
