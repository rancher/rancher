package wrapper

import (
	"testing"

	"github.com/rancher/norman/api/handler"
	"github.com/rancher/norman/parse"
	"github.com/rancher/norman/store/empty"
	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
)

type testStore struct {
	empty.Store
}

func (t *testStore) List(apiContext *types.APIContext, schema *types.Schema, opt *types.QueryOptions) ([]map[string]interface{}, error) {
	return []map[string]interface{}{{"1": "1"}, {"2": "2"}, {"3": "3"}}, nil
}

func TestWrap(t *testing.T) {
	store := &testStore{}
	limit := int64(1)
	opt := &types.QueryOptions{
		Pagination: &types.Pagination{
			Limit: &limit,
		},
	}
	apiContext := &types.APIContext{
		SubContextAttributeProvider: &parse.DefaultSubContextAttributeProvider{},
		QueryFilter:                 handler.QueryFilter,
		Pagination:                  opt.Pagination,
	}

	wrapped := Wrap(store)
	if _, err := wrapped.List(apiContext, &types.Schema{}, opt); err != nil {
		t.Fatal(err)
	}
	assert.True(t, apiContext.Pagination.Partial)
	assert.Equal(t, int64(3), *apiContext.Pagination.Total)
	assert.Equal(t, int64(1), *apiContext.Pagination.Limit)

	wrappedTwice := Wrap(wrapped)
	apiContext.Pagination = opt.Pagination
	if _, err := wrappedTwice.List(apiContext, &types.Schema{}, opt); err != nil {
		t.Fatal(err)
	}
	assert.True(t, apiContext.Pagination.Partial)
	assert.Equal(t, int64(3), *apiContext.Pagination.Total)
	assert.Equal(t, int64(1), *apiContext.Pagination.Limit)

}
