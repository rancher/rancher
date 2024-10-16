package stores

import (
	steveext "github.com/rancher/steve/pkg/ext"
	"k8s.io/apimachinery/pkg/runtime"
)

func InstallStores(server *steveext.ExtensionAPIServer, scheme *runtime.Scheme) error {
	steveext.AddToScheme(scheme)

	// To add a store to the extensionAPIServer, simply call add the types to the *runtime.Scheme and
	// call InstallStore with the required fields.
	//
	// Here's an example:
	//
	//	extv1.AddToScheme(scheme)
	//	store := newMapStore()
	//	err := steveext.InstallStore(server, &extv1.TestType{}, &extv1.TestTypeList{}, "testtypes", "testtype", extv1.SchemeGroupVersion.WithKind("TestType"), store)
	//	if err != nil {
	//		return fmt.Errorf("unable to install mapStore: %w", err)
	//	}

	return nil
}
