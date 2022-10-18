package nsserviceaccount

import (
	"context"

	rv1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	projectIDAnnotation        = "field.cattle.io/projectId"
	sysNamespaceAnnotation     = "management.cattle.io/system-namespace"
	NoDefaultSATokenAnnotation = "management.cattle.io/no-default-sa-token"
)

type defaultSvcAccountHandler struct {
	serviceAccountsLister rv1.ServiceAccountLister
	serviceAccounts       rv1.ServiceAccountInterface
}

func Register(ctx context.Context, cluster *config.UserOnlyContext) {
	logrus.Debugf("Registering defaultSvcAccountHandler for checking default service account of system namespaces")
	nsh := &defaultSvcAccountHandler{
		serviceAccounts:       cluster.Core.ServiceAccounts(""),
		serviceAccountsLister: cluster.Core.ServiceAccounts("").Controller().Lister(),
	}
	cluster.Core.Namespaces("").AddHandler(ctx, "defaultSvcAccountHandler", nsh.Sync)
}

func (nsh *defaultSvcAccountHandler) Sync(key string, ns *corev1.Namespace) (runtime.Object, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Debugf("defaultSvcAccountHandler: Sync service account: key=%v", key)
	//handle default svcAccount of system namespaces only
	if err := nsh.handleIfSystemNSDefaultSA(ns); err != nil {
		logrus.Errorf("defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=%v, err=%v", key, err)
	}
	return nil, nil
}

func (nsh *defaultSvcAccountHandler) handleIfSystemNSDefaultSA(ns *corev1.Namespace) error {
	if ns.Annotations[NoDefaultSATokenAnnotation] != "true" {
		return nil
	}

	defSvcAccnt, err := nsh.serviceAccountsLister.Get(ns.Name, "default")
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	if defSvcAccnt.AutomountServiceAccountToken != nil && *defSvcAccnt.AutomountServiceAccountToken == false {
		return nil
	}
	automountServiceAccountToken := false
	defSvcAccnt.AutomountServiceAccountToken = &automountServiceAccountToken
	logrus.Debugf("defaultSvcAccountHandler: updating default service account key=%v", defSvcAccnt)
	_, err = nsh.serviceAccounts.Update(defSvcAccnt)
	if err != nil {
		logrus.Errorf("defaultSvcAccountHandler: error updating default service account flag for namespace: %v, err=%+v", ns.Name, err)
		return err
	}
	return nil
}
