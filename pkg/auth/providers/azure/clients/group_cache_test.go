package clients

import (
	"testing"

	lru "github.com/hashicorp/golang-lru"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestUserGroupsToPrincipals(t *testing.T) {
	setupTestCache(t)
	testGUID := "0f8fad5b-d9cb-469f-a165-70867728950e"

	fc := &fakePrincipalsClient{
		groups: map[string]fakeGroup{
			testGUID: fakeGroup{id: ptr.To(testGUID)},
		},
	}
	principals, err := UserGroupsToPrincipals(fc, []string{testGUID})
	require.NoError(t, err)

	want := []v3.Principal{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "azuread_group://0f8fad5b-d9cb-469f-a165-70867728950e",
			},
			PrincipalType: "group",
			MemberOf:      true,
			Provider:      "azuread",
		},
	}
	assert.Equal(t, want, principals)
}

type fakePrincipalsClient struct {
	groups map[string]fakeGroup
}

func (f *fakePrincipalsClient) GetGroup(id string) (v3.Principal, error) {
	return groupToPrincipal(f.groups[id]), nil
}

type fakeGroup struct {
	id          *string
	displayName *string
}

func (f fakeGroup) GetId() *string {
	return f.id
}

func (f fakeGroup) GetDisplayName() *string {
	return f.displayName
}

func setupTestCache(t *testing.T) {
	t.Helper()
	oldGroupCache := GroupCache
	t.Cleanup(func() {
		GroupCache = oldGroupCache
	})
	gc, err := lru.New(10)
	require.NoError(t, err)
	GroupCache = gc
}
