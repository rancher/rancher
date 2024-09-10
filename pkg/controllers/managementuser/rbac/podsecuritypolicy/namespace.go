package podsecuritypolicy

import (
	"context"
	"errors"
	"fmt"

	v12 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type namespaceManager struct {
	serviceAccountsController v12.ServiceAccountController
	serviceAccountLister      v12.ServiceAccountLister
	clusterLister             v3.ClusterLister
	clusterName               string
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
		clusterLister:             context.Management.Management.Clusters("").Controller().Lister(),
		clusterName:               context.ClusterName,
	}

	context.Core.Namespaces("").AddHandler(ctx, "NamespaceSyncHandler", m.sync)
}

func (m *namespaceManager) sync(key string, obj *v1.Namespace) (runtime.Object, error) {
	if obj == nil || obj.DeletionTimestamp != nil ||
		obj.Status.Phase == v1.NamespaceTerminating {
		return nil, nil
	}

	err := CheckClusterVersion(m.clusterName, m.clusterLister)
	if err != nil {
		if errors.Is(err, ErrClusterVersionIncompatible) {
			return obj, nil
		}
		return obj, fmt.Errorf("error checking cluster version for Namespace controller: %w", err)
	}

	return nil, resyncServiceAccounts(m.serviceAccountLister, m.serviceAccountsController, obj.Name)
}
