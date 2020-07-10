package roletemplate

import (
	"testing"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type dummyStore struct{}

// testWithInherited attempt delete given key and inserts the key in  the inherited
// list for a roletemplate if insertInLister is true.
func testWithInherited(key string, insertInLister bool) (map[string]interface{}, error) {
	insert := "randomstringnotusedkey"
	if insertInLister {
		insert = key
	}
	mockRTLister := &fakes.RoleTemplateListerMock{
		ListFunc: mockList(insert),
	}

	testStore := Wrap(dummyStore{}, mockRTLister)

	return testStore.Delete(nil, nil, key)
}

func mockList(insert string) func(namespace string, selector labels.Selector) ([]*v3.RoleTemplate, error) {
	return func(namespace string, selector labels.Selector) ([]*v3.RoleTemplate, error) {
		return []*v3.RoleTemplate{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "rt1",
				},
				RoleTemplateNames: []string{"asdf", "asdf"},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "rt2",
				},
				RoleTemplateNames: []string{"aasdfsdf", insert, "adsasdff"},
			},
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "rt3",
				},
				RoleTemplateNames: []string{},
			}}, nil
	}
}

// TestDelete confirms that the store prevents a roletemplate from being deleted if another roletemplate inherits from it
func TestDelete(t *testing.T) {
	assert := assert.New(t)

	// test when roletemplate is not a parent of another roletemplate, should not cause error
	_, err := testWithInherited("somekey", false)
	assert.Nil(err)

	// test when roletemplate is a parent of another roletemplate, should cause error
	_, err = testWithInherited("somekey", true)
	assert.Contains(err.Error(), "Conflict 409: roletemplate [somekey] cannot be deleted because roletemplate [rt2] inherits from it")
}

func (ds dummyStore) Context() types.StorageContext {
	return types.StorageContext("test")
}
func (ds dummyStore) ByID(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, nil
}
func (ds dummyStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	return nil, nil
}

func (ds dummyStore) Create(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}) (map[string]interface{}, error) {
	return data, nil
}

func (ds dummyStore) Update(apiContext *types.APIContext, schema *types.Schema, data map[string]interface{}, id string) (map[string]interface{}, error) {
	return nil, nil
}

func (ds dummyStore) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	return nil, nil
}

func (ds dummyStore) Watch(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) (chan map[string]interface{}, error) {
	return nil, nil
}
