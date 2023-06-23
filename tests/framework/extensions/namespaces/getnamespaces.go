package namespaces

import (
	"sort"

	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
)

// ListNamespaceNames is a helper which returns a sorted list of namespace names
func ListNamespaceNames(steveclient *v1.Client) ([]string, error) {

	namespaceList, err := steveclient.SteveType(NamespaceSteveType).List(nil)
	if err != nil {
		return nil, err
	}

	namespace := make([]string, len(namespaceList.Data))
	for idx, ns := range namespaceList.Data {
		namespace[idx] = ns.GetName()
	}
	sort.Strings(namespace)
	return namespace, nil
}
