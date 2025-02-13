package stores

import (
	"fmt"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/ext/stores/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	steveext "github.com/rancher/steve/pkg/ext"
	"k8s.io/apimachinery/pkg/runtime"
)

func InstallStores(
	server *steveext.ExtensionAPIServer,
	wranglerContext *wrangler.Context,
	scheme *runtime.Scheme,
) error {
	steveext.AddToScheme(scheme)

	// To add a store to the extensionAPIServer, simply add the types to the *runtime.Scheme and
	// call InstallStore [steveext.ExtensionAPIServer.Install].
	//
	// Here's an example:
	//
	//	extv1.AddToScheme(scheme)
	//
	//      authorizer := server.GetAuthorizer()
	//	store := newMapStore(authorizer)
	//
	//      err := server.Install("testtypes", extv1.SchemeGroupVersion.WithKind("TestType"), store)
	//	if err != nil {
	//		return fmt.Errorf("unable to install mapStore: %w", err)
	//	}

	extv1.AddToScheme(scheme)

	userManager, err := common.NewUserManagerNoBindings(wranglerContext)
	if err != nil {
		return fmt.Errorf("error getting user manager: %w", err)
	}

	if err := server.Install(
		extv1.KubeconfigResourceName,
		extv1.SchemeGroupVersion.WithKind(kubeconfig.Kind),
		kubeconfig.New(wranglerContext, server.GetAuthorizer(), userManager),
	); err != nil {
		return fmt.Errorf("unable to install kubeconfig store: %w", err)
	}

	return nil
}
