package local

import (
	"slices"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func TestProvider_SearchPrincipals(t *testing.T) {
	provider := Provider{
		userIndexer: newTestUserIndexer(&v3.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-12345",
			},
			Username:    "test",
			DisplayName: "Test User",
		}),
		groupIndexer: newTestGroupIndexer(),
	}

	principals, err := provider.SearchPrincipals("User", "user", v3.Token{})
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, p := range principals {
		names = append(names, p.Name)
	}
	want := []string{"local://u-12345"}
	if !slices.Equal(names, want) {
		t.Errorf("SearchPrincipals() got %#v, want %#v", names, want)
	}
}

func newTestUserIndexer(indexed ...*v3.User) cache.Indexer {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		userSearchIndex: userSearchIndexer,
	})

	for i := range indexed {
		indexer.Add(indexed[i])
	}

	return indexer
}

func newTestGroupIndexer(indexed ...v3.Group) cache.Indexer {
	return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{
		groupSearchIndex: groupSearchIndexer,
	})
}
