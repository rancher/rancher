package stores

import (
	"fmt"

	extv1 "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/rancher/rancher/pkg/ext/stores/tokens"
	"github.com/rancher/rancher/pkg/wrangler"

	steveext "github.com/rancher/steve/pkg/ext"
	"k8s.io/apimachinery/pkg/runtime"
)

func InstallStores(server *steveext.ExtensionAPIServer, scheme *runtime.Scheme) error {
	steveext.AddToScheme(scheme)

	// To add a store to the extensionAPIServer, simply add the types to the *runtime.Scheme and
	// call InstallStore with the required fields.

	extv1.AddToScheme(scheme)

	fakeWrangler := &wrangler.Context{}
	tokenStore := tokens.NewTokenStoreFromWrangler(fakeWrangler)

	err := steveext.InstallStore(
		server,
		&extv1.Token{},
		&extv1.TokenList{},
		"tokens",
		"token",
		extv1.SchemeGroupVersion.WithKind("Token"),
		tokenStore)
	if err != nil {
		return fmt.Errorf("unable to install tokenStore: %w", err)
	}

	return nil
}
