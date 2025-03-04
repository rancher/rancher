package stores

import (
	"fmt"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/ext/stores/useractivity"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

func InstallStores(server *steveext.ExtensionAPIServer, wranglerContext *wrangler.Context, scheme *runtime.Scheme) error {
	steveext.AddToScheme(scheme)
	extv1.AddToScheme(scheme)

	err := server.Install(extv1.UserActivityResourceName, useractivity.GVK, useractivity.New(wranglerContext))
	if err != nil {
		return fmt.Errorf("unable to install useractivity store: %w", err)
	}

	logrus.Infof("Installing ext token store")
	extv1.AddToScheme(scheme)
	err = server.Install(
		tokens.PluralName,
		tokens.GVK,
		tokens.NewFromWrangler(wranglerContext, server.GetAuthorizer()))
	if err != nil {
		return fmt.Errorf("unable to install %s store: %w", tokens.SingularName, err)
	}
	logrus.Infof("Successfully installed ext token store")

	return nil
}
