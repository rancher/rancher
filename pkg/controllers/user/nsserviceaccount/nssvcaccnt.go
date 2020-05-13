package nsserviceaccount

import (
	"context"
	"strings"

	"github.com/rancher/rancher/pkg/settings"
	rv1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type namespaceSvcAccountHandler struct {
	serviceAccountLister rv1.ServiceAccountLister
	serviceAccounts      rv1.ServiceAccountInterface
}

func Register(ctx context.Context, cluster *config.UserContext) {
	logrus.Debugf("Registering namespaceSvcAccountHandler for checking default serviceaccount")
	nsh := &namespaceSvcAccountHandler{
		serviceAccountLister: cluster.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccounts:      cluster.Core.ServiceAccounts(""),
	}
	cluster.Core.Namespaces("").AddHandler(ctx, "namespaceSvcAccountHandler", nsh.Sync)
}

func (nsh *namespaceSvcAccountHandler) Sync(key string, ns *corev1.Namespace) (runtime.Object, error) {
	if ns == nil || ns.DeletionTimestamp != nil {
		return nil, nil
	}
	logrus.Debugf("namespaceSvcAccountHandler: Sync namespace: key=%v", key)

	//handle default svcAccount of system namespaces
	if err := nsh.handleSystemNS(key); err != nil {
		logrus.Errorf("namespaceSvcAccountHandler: Sync: error handling default ServiceAccount of namespace key=%v, err=%v", key, err)
	}
	return nil, nil
}

func (nsh *namespaceSvcAccountHandler) handleSystemNS(namespace string) error {
	if namespace == "kube-system" || namespace == "default" || !nsh.isSystemNS(namespace) {
		return nil
	}
	defSvcAccnt, err := nsh.serviceAccountLister.Get(namespace, "default")
	if err != nil {
		logrus.Errorf("namespaceSvcAccountHandler: error listing serviceaccount flag: Sync: key=%v, err=%+v", namespace, err)
		return err
	}

	if defSvcAccnt.AutomountServiceAccountToken != nil && *defSvcAccnt.AutomountServiceAccountToken == false {
		return nil
	}

	defSvcAccntCopy := defSvcAccnt.DeepCopy()
	automountServiceAccountToken := false
	defSvcAccntCopy.AutomountServiceAccountToken = &automountServiceAccountToken
	logrus.Debugf("namespaceSvcAccountHandler: updating default serviceaccount key=%v", defSvcAccntCopy)
	_, err = nsh.serviceAccounts.Update(defSvcAccntCopy)
	if err != nil {
		logrus.Errorf("namespaceSvcAccountHandler: error updating serviceaccnt flag: Sync: key=%v, err=%+v", namespace, err)
		return err
	}
	return nil
}

func (nsh *namespaceSvcAccountHandler) isSystemNS(namespace string) bool {
	systemNamespacesStr := settings.SystemNamespaces.Get()
	if systemNamespacesStr == "" {
		return false
	}

	systemNamespaces := make(map[string]bool)
	splitted := strings.Split(systemNamespacesStr, ",")
	for _, s := range splitted {
		ns := strings.TrimSpace(s)
		systemNamespaces[ns] = true
	}

	return systemNamespaces[namespace]
}
