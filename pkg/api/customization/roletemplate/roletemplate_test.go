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

// testAsRoleTemplate creates resource with ID set to key and runs roletemplate formatter
func testAsRoletemplate(key string) *types.RawResource {
	mockRTLister := &fakes.RoleTemplateListerMock{
		ListFunc: mockList,
	}

	testWrapper := Wrapper{
		mockRTLister,
	}

	testResource := &types.RawResource{
		ID: key,
		Links: map[string]string{
			"update": "/test/link",
			"remove": "/test/link2",
		},
	}

	testWrapper.Formatter(nil, testResource)

	return testResource
}

func mockList(namespace string, selector labels.Selector) ([]*v3.RoleTemplate, error) {
	return []*v3.RoleTemplate{
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "rt1",
			},
			RoleTemplateNames: []string{"asdf", "asdf", "test123"},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "rt2",
			},
			RoleTemplateNames: []string{"aasdfsdf", "adsasdff"},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "rt3",
			},
			RoleTemplateNames: []string{},
		},
		{
			ObjectMeta: v1.ObjectMeta{
				Name: "rt3",
			},
			RoleTemplateNames: []string{"test321"},
		}}, nil
}

// TestDelete confirms that the remove resource link is filtered when roletemplate is inherited
func TestDelete(t *testing.T) {
	assert := assert.New(t)

	// test when roletemplate is not a parent of another roletemplate, should have all links
	resource := testAsRoletemplate("somekey")
	assert.Equal(resource.Links["remove"], "/test/link2")
	assert.Equal(resource.Links["update"], "/test/link")

	// test when roletemplate is a parent of another roletemplate, should not have remove link
	resource = testAsRoletemplate("test123")
	assert.Equal(resource.Links["remove"], "")
	assert.Equal(resource.Links["update"], "/test/link")

	// test when roletemplate is only parent of another roletemplate, should not have remove link
	resource = testAsRoletemplate("test321")
	assert.Equal(resource.Links["remove"], "")
	assert.Equal(resource.Links["update"], "/test/link")
}
