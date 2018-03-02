package podsecuritypolicy

import (
	v12 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

type namespaceManager struct {
	serviceAccountsController v12.ServiceAccountController
	serviceAccountLister      v12.ServiceAccountLister
}

// RegisterNamespace resyncs the current namespace's service accounts.  This is necessary because service accounts
// determine their parent project via an annotation on the namespace, and the namespace is not always present when the
// service account handler is triggered.  So we have this handler to retrigger the serviceaccount handler once the
// annotation has been added.
func RegisterNamespace(context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy namespace handler for cluster %v", context.ClusterName)

	m := &namespaceManager{
		serviceAccountLister:      context.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccountsController: context.Core.ServiceAccounts("").Controller(),
	}

	context.Core.Namespaces("").AddHandler("NamespaceSyncHandler", m.sync)
}

func (m *namespaceManager) sync(key string, obj *v1.Namespace) error {
	if obj == nil {
		return nil
	}

	return resyncServiceAccounts(m.serviceAccountLister, m.serviceAccountsController, obj.Name)
}
