package local

import (
	"sort"
	"testing"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func TestProviderSearchPrincipal(t *testing.T) {
	testUsers := []*v3.User{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-12345",
			},
			Username:    "test",
			DisplayName: "Test User",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-23456",
			},
			Username:    "other",
			DisplayName: "Other User",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-34567",
			},
			Username:    "otter",
			DisplayName: "Significant Otter",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-45678",
			},
			Username:    "johns",
			DisplayName: "John Smith",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-56789",
			},
			Username:    "edubois",
			DisplayName: "Émile Dubois",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-67890",
			},
			Username:    "jalicia",
			DisplayName: "Alicia Johns",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "u-7890a",
			},
			Username:    "jalice",
			DisplayName: "Alice Johnson",
		},
	}

	provider := Provider{
		userLister:   fakeUserLister{users: testUsers},
		userIndexer:  newTestUserIndexer(testUsers...),
		groupIndexer: newTestGroupIndexer(),
		groupLister:  fakeGroupLister{},
	}

	principalSearchTests := []struct {
		searchKey string
		want      []string
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
			searchKey: "Test U",
			want:      []string{"local://u-12345"},
		},
		{
			searchKey: "test U",
			want:      []string{"local://u-12345"},
		},
		{
			searchKey: "test u",
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
		{
			searchKey: "émile",
			want:      []string{"local://u-56789"},
		},
		{
			searchKey: "Emile",
			want:      []string{"local://u-56789"},
		},
		{
			searchKey: "emile",
			want:      []string{"local://u-56789"},
		},
		{
			searchKey: "Émile Dubois",
			want:      []string{"local://u-56789"},
		},
		{
			searchKey: "émile dubois",
			want:      []string{"local://u-56789"},
		},

		{
			searchKey: "test user",
			want:      []string{"local://u-12345"},
		},
		{
			searchKey: "testuser",
			want:      []string{"local://u-12345"},
		},
		{
			searchKey: "Emile Dubois",
			want:      []string{"local://u-56789"},
		},
	}

	for _, tt := range principalSearchTests {
		t.Run("searchKey "+tt.searchKey, func(t *testing.T) {
			principals, err := provider.SearchPrincipals(tt.searchKey, "user", &v3.Token{})
			require.NoError(t, err)

			var names []string
			for _, p := range principals {
				names = append(names, p.Name)
			}

			sort.Strings(names)
			sort.Strings(tt.want)

			require.Equal(t, tt.want, names)
		})

		// and the same behaviour for ext tokens
		t.Run("searchKey "+tt.searchKey+", ext ", func(t *testing.T) {
			principals, err := provider.SearchPrincipals(tt.searchKey, "user", &ext.Token{})
			require.NoError(t, err)

			var names []string
			for _, p := range principals {
				names = append(names, p.Name)
			}

			sort.Strings(names)
			sort.Strings(tt.want)

			require.Equal(t, tt.want, names)
		})
	}
}

func TestUserSearchIndexer(t *testing.T) {
	indexerTests := []struct {
		user        *v3.User
		wantIndexed []string
	}{
		{
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-12345",
				},
				Username:    "test",
				DisplayName: "Test User",
			},
			wantIndexed: []string{
				"te", "tes", "test", "test ", "test u",
				"u-", "u-1", "u-12", "u-123", "u-1234", "user",
			},
		},
		{
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-56789",
				},
				Username:    "edubois",
				DisplayName: "Émile Dubois",
			},
			wantIndexed: []string{
				"dubois", "ed", "edu", "edub", "edubo",
				"eduboi", "em", "emi", "emil", "emile", "emile ", "u-",
				"u-5", "u-56", "u-567", "u-5678", "émile",
			},
		},
		{
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-67890",
				},
				Username:    "jalicia",
				DisplayName: "Alicia Johns",
			},
			wantIndexed: []string{
				"al", "ali", "alic", "alici", "alicia",
				"ja", "jal", "jali", "jalic", "jalici", "johns", "u-",
				"u-6", "u-67", "u-678", "u-6789",
			},
		},
		{
			// Non-Unicode string lower-cased to ASCII.
			user: &v3.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "u-99900",
				},
				Username:    "\u212ael",
				DisplayName: "\u212aelvin Smith",
			},
			wantIndexed: []string{"ke", "kel", "kelv", "kelvi", "kelvin", "smith", "u-", "u-9", "u-99", "u-999", "u-9990"},
		},
	}

	for _, tt := range indexerTests {
		t.Run(tt.user.Name, func(t *testing.T) {
			indexed, err := userSearchIndexer(tt.user)
			require.NoError(t, err)

			sort.Strings(indexed)
			sort.Strings(tt.wantIndexed)

			require.Equal(t, tt.wantIndexed, indexed)
		})
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
