package steve

import v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"

// Names is a public function that accepts SteveCollection pointer as a parameter,
// and returns each item name in the list as a new slice of strings.
func Names(collection *v1.SteveCollection) (names []string) {

	for _, item := range collection.Data {
		names = append(names, item.Name)
	}

	return
}
