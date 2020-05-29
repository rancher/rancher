package nsserviceaccount

import (
	"context"
	"strings"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/settings"
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type defaultSvcAccountHandler struct {
	serviceAccounts rv1.ServiceAccountInterface
}

func Register(ctx context.Context, cluster *config.UserContext) {
	logrus.Debugf("Registering defaultSvcAccountHandler for checking default service account of system namespaces")
	nsh := &defaultSvcAccountHandler{
		serviceAccounts: cluster.Core.ServiceAccounts(""),
	}
	cluster.Core.ServiceAccounts("").AddHandler(ctx, "defaultSvcAccountHandler", nsh.Sync)
}

func (nsh *defaultSvcAccountHandler) Sync(key string, sa *corev1.ServiceAccount) (runtime.Object, error) {
	if sa == nil || sa.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Debugf("defaultSvcAccountHandler: Sync service account: key=%v", key)
	//handle default svcAccount of system namespaces only
	if err := nsh.handleIfSystemNSDefaultSA(sa); err != nil {
		logrus.Errorf("defaultSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=%v, err=%v", key, err)
	}
	return nil, nil
}

func (nsh *defaultSvcAccountHandler) handleIfSystemNSDefaultSA(defSvcAccnt *corev1.ServiceAccount) error {
	//check if sa is "default"
	if defSvcAccnt.Name != "default" {
		return nil
	}
	//check if ns is a system-ns
	namespace := defSvcAccnt.Namespace
	if namespace == "kube-system" || namespace == "default" || !nsh.isSystemNS(namespace) {
		return nil
	}
	if defSvcAccnt.AutomountServiceAccountToken != nil && *defSvcAccnt.AutomountServiceAccountToken == false {
		return nil
	}
	automountServiceAccountToken := false
	defSvcAccnt.AutomountServiceAccountToken = &automountServiceAccountToken
	logrus.Debugf("defaultSvcAccountHandler: updating default service account key=%v", defSvcAccnt)
	_, err := nsh.serviceAccounts.Update(defSvcAccnt)
	if err != nil {
		logrus.Errorf("defaultSvcAccountHandler: error updating default service account flag for namespace: %v, err=%+v", namespace, err)
		return err
	}
	return nil
}

func (nsh *defaultSvcAccountHandler) isSystemNS(namespace string) bool {
	systemNamespacesStr := settings.SystemNamespaces.Get()
	if systemNamespacesStr == "" {
		return false
	}
	systemNamespaces := strings.Split(systemNamespacesStr, ",")
	return slice.ContainsString(systemNamespaces, namespace)
}
