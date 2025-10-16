package stores

import (
	"fmt"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/ext/stores/groupmembershiprefreshrequest"
	"github.com/rancher/rancher/pkg/ext/stores/kubeconfig"
	"github.com/rancher/rancher/pkg/ext/stores/passwordchangerequest"
	"github.com/rancher/rancher/pkg/ext/stores/selfuser"
	"github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/ext/stores/useractivity"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

func InstallStores(
	server *steveext.ExtensionAPIServer,
	wranglerContext *wrangler.Context,
	scheme *runtime.Scheme,
) error {
	steveext.AddToScheme(scheme)
	extv1.AddToScheme(scheme)

	err := server.Install(
		extv1.UserActivityResourceName,
		useractivity.GVK,
		useractivity.New(wranglerContext, server.GetAuthorizer()),
	)
	if err != nil {
		return fmt.Errorf("unable to install useractivity store: %w", err)
	}
	logrus.Infof("Successfully installed useractivity store")

	if err := server.Install(
		tokens.PluralName,
		tokens.GVK,
		tokens.NewFromWrangler(wranglerContext, server.GetAuthorizer()),
	); err != nil {
		return fmt.Errorf("unable to install %s store: %w", tokens.SingularName, err)
	}
	logrus.Infof("Successfully installed %s store", tokens.SingularName)

	if err := server.Install(
		extv1.KubeconfigResourceName,
		extv1.SchemeGroupVersion.WithKind(kubeconfig.Kind),
		kubeconfig.New(features.MCM.Enabled(), wranglerContext, server.GetAuthorizer()),
	); err != nil {
		return fmt.Errorf("unable to install %s store: %w", kubeconfig.Singular, err)
	}
	logrus.Infof("Successfully installed %s store", kubeconfig.Singular)

	if err = server.Install(
		extv1.PasswordChangeRequestResourceName,
		passwordchangerequest.GVK,
		passwordchangerequest.New(wranglerContext, server.GetAuthorizer()),
	); err != nil {
		return fmt.Errorf("unable to install %s store: %w", passwordchangerequest.SingularName, err)
	}
	logrus.Infof("Successfully installed %s store", passwordchangerequest.SingularName)

	groupMembershipRefreshStore, err := groupmembershiprefreshrequest.New(wranglerContext, server.GetAuthorizer())
	if err != nil {
		return fmt.Errorf("unable to create %s store: %w", groupmembershiprefreshrequest.SingularName, err)
	}

	if err = server.Install(
		extv1.GroupMembershipRefreshRequestResourceName,
		groupmembershiprefreshrequest.GVK,
		groupMembershipRefreshStore,
	); err != nil {
		return fmt.Errorf("unable to install %s store: %w", groupmembershiprefreshrequest.SingularName, err)
	}
	logrus.Infof("Successfully installed %s store", groupmembershiprefreshrequest.SingularName)

	if err = server.Install(
		extv1.SelfUserResourceName,
		selfuser.GVK,
		selfuser.New(),
	); err != nil {
		return fmt.Errorf("unable to install %s store: %w", selfuser.SingularName, err)
	}
	logrus.Infof("Successfully installed %s store", selfuser.SingularName)

	return nil
}
