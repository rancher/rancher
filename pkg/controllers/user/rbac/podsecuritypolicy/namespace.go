package podsecuritypolicy

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	v12 "github.com/rancher/rancher/pkg/types/apis/core/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

type namespaceManager struct {
	serviceAccountsController v12.ServiceAccountController
	serviceAccountLister      v12.ServiceAccountLister
}

// RegisterNamespace resyncs the current namespace's service accounts.  This is necessary because service accounts
// determine their parent project via an annotation on the namespace, and the namespace is not always present when the
// service account handler is triggered.  So we have this handler to retrigger the serviceaccount handler once the
// annotation has been added.
func RegisterNamespace(ctx context.Context, context *config.UserContext) {
	logrus.Infof("registering podsecuritypolicy namespace handler for cluster %v", context.ClusterName)

	m := &namespaceManager{
		serviceAccountLister:      context.Core.ServiceAccounts("").Controller().Lister(),
		serviceAccountsController: context.Core.ServiceAccounts("").Controller(),
	}

	context.Core.Namespaces("").AddHandler(ctx, "NamespaceSyncHandler", m.sync)
}

func (m *namespaceManager) sync(key string, obj *v1.Namespace) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil ||
		obj.Status.Phase == v1.NamespaceTerminating {
		return nil, nil
	}

	return nil, resyncServiceAccounts(m.serviceAccountLister, m.serviceAccountsController, obj.Name)
}
