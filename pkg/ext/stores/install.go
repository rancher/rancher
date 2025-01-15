package stores

import (
	steveext "github.com/rancher/steve/pkg/ext"
	"k8s.io/apimachinery/pkg/runtime"
)

func InstallStores(server *steveext.ExtensionAPIServer, scheme *runtime.Scheme) error {
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

	return nil
}
