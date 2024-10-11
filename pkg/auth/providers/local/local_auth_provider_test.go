package local

import (
	"fmt"
	"slices"
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func TestProvider_SearchPrincipals_short_names(t *testing.T) {
	// if the query string is less than searchIndexDefaultLen
	// we query the indexer.
	// Longer queries use the userLister and match
	provider := Provider{
		userIndexer: newTestUserIndexer(
			&v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-12345",
				},
				Username:    "test",
				DisplayName: "Test User",
			},
			&v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-23456",
				},
				Username:    "other",
				DisplayName: "Other User",
			},
			&v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-34567",
				},
				Username:    "otter",
				DisplayName: "Significant Otter",
			},
			&v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-45678",
				},
				Username:    "johns",
				DisplayName: "John Smith",
			},
		),
		groupIndexer: newTestGroupIndexer(), // only needed to prevent nil-pointer.
	}

	shortNameTests := []struct {
		searchKey string

		want []string
	}{
		{
			searchKey: "tes",
			want:      []string{"local://u-12345"},
		},
		{
			searchKey: "Tes",
			want:      []string{"local://u-12345"},
		},

		{
			searchKey: "oth",
			want:      []string{"local://u-23456"},
		},
		{
			searchKey: "ot",
			want:      []string{"local://u-23456", "local://u-34567"},
		},
		{
			searchKey: "Smith",
			want:      []string{"local://u-45678"},
		},
		{
			searchKey: "smith",
			want:      []string{"local://u-45678"},
		},
		{
			searchKey: "John",
			want:      []string{"local://u-45678"},
		},
		{
			searchKey: "john",
			want:      []string{"local://u-45678"},
		},
	}

	for _, tt := range shortNameTests {
		t.Run(fmt.Sprintf("searchKey = %s", tt.searchKey), func(t *testing.T) {
			principals, err := provider.SearchPrincipals(tt.searchKey, "user", v3.Token{})
			if err != nil {
				t.Fatal(err)
			}

			var names []string
			for _, p := range principals {
				names = append(names, p.Name)
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("SearchPrincipals() got %#v, want %#v", names, tt.want)
			}
		})
	}
}

func TestProvider_SearchPrincipals_long_search(t *testing.T) {
	provider := Provider{
		userLister: fakeUserLister{
			users: []*v3.User{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "u-12345",
					},
					Username:    "test",
					DisplayName: "Test User",
				},
			},
		},
		groupLister:  fakeGroupLister{},
		userIndexer:  newTestUserIndexer(),
		groupIndexer: newTestGroupIndexer(),
	}

	principals, err := provider.SearchPrincipals("test user", "user", v3.Token{})
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

type fakeUserLister struct {
	users []*v3.User
}

func (f fakeUserLister) List(namespace string, selector labels.Selector) ([]*v3.User, error) {
	return f.users, nil
}
func (f fakeUserLister) Get(namespace, name string) (*v3.User, error) {
	return nil, nil
}

type fakeGroupLister struct {
}

func (f fakeGroupLister) List(namespace string, selector labels.Selector) ([]*v3.Group, error) {
	return nil, nil
}
func (f fakeGroupLister) Get(namespace, name string) (*v3.Group, error) {
	return nil, nil
}
