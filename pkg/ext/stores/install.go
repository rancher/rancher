package stores

import (
	"fmt"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/ext/stores/kubeconfig"
	"github.com/rancher/rancher/pkg/ext/stores/passwordchangerequest"
	"github.com/rancher/rancher/pkg/ext/stores/selfuser"
	"github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/ext/stores/useractivity"
	"github.com/rancher/rancher/pkg/ext/stores/userrefreshrequest"
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
		useractivity.New(wranglerContext),
	)
	if err != nil {
		return fmt.Errorf("unable to install useractivity store: %w", err)
	}
	logrus.Infof("Successfully installed useractivity store")

	if features.ExtTokens.Enabled() {
		if err := server.Install(
			tokens.PluralName,
			tokens.GVK,
			tokens.NewFromWrangler(wranglerContext, server.GetAuthorizer()),
		); err != nil {
			return fmt.Errorf("unable to install %s store: %w", tokens.SingularName, err)
		}
		logrus.Infof("Successfully installed token store")
	} else {
		logrus.Infof("Feature ext-tokens is disabled")
	}

	if features.ExtKubeconfigs.Enabled() {
		userManager, err := common.NewUserManagerNoBindings(wranglerContext)
		if err != nil {
			return fmt.Errorf("error getting user manager: %w", err)
		}

		if err := server.Install(
			extv1.KubeconfigResourceName,
			extv1.SchemeGroupVersion.WithKind(kubeconfig.Kind),
			kubeconfig.New(features.MCM.Enabled(), wranglerContext, server.GetAuthorizer(), userManager),
		); err != nil {
			return fmt.Errorf("unable to install kubeconfig store: %w", err)
		}
		logrus.Infof("Successfully installed kubeconfig store")
	} else {
		logrus.Infof("Feature ext-kubeconfigs is disabled")
	}

	err = server.Install(
		passwordchangerequest.PluralName,
		passwordchangerequest.GVK,
		passwordchangerequest.New(wranglerContext, server.GetAuthorizer()))
	if err != nil {
		return fmt.Errorf("unable to install %s store: %w", passwordchangerequest.SingularName, err)
	}
	userRefreshStore, err := userrefreshrequest.New(wranglerContext, server.GetAuthorizer())
	if err != nil {
		return fmt.Errorf("unable to create %s store: %w", userrefreshrequest.SingularName, err)
	}
	err = server.Install(
		userrefreshrequest.PluralName,
		userrefreshrequest.GVK,
		userRefreshStore)
	if err != nil {
		return fmt.Errorf("unable to install %s store: %w", userrefreshrequest.SingularName, err)
	}
	err = server.Install(
		selfuser.PluralName,
		selfuser.GVK,
		selfuser.New())
	if err != nil {
		return fmt.Errorf("unable to install %s store: %w", selfuser.SingularName, err)
	}

	return nil
}
