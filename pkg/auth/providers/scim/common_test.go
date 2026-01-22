package scim

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirst(t *testing.T) {
	t.Run("strings", func(t *testing.T) {
		var s []string
		assert.Equal(t, "", first(s))

		s = []string{"a", "b", "c"}
		assert.Equal(t, "a", first(s))
	})

	t.Run("ints", func(t *testing.T) {
		var s []int
		assert.Equal(t, 0, first(s))

		s = []int{1, 2, 3}
		assert.Equal(t, 1, first(s))
	})

	t.Run("structs", func(t *testing.T) {
		type T struct {
			A string
			b []int
		}
		var s []T
		var zero T
		assert.Equal(t, zero, first(s))

		s = []T{
			{A: "first", b: []int{1, 2}},
			{A: "second", b: []int{3, 4}},
		}
		assert.Equal(t, T{A: "first", b: []int{1, 2}}, first(s))
	})
}

func TestParsePaginationParams(t *testing.T) {
	tests := []struct {
		name        string
		queryString string
		want        PaginationParams
		wantErr     string
	}{
		{
			name:        "default values when no params",
			queryString: "",
			want:        PaginationParams{StartIndex: 1, Count: DefaultPageSize},
		},
		{
			name:        "custom startIndex",
			queryString: "startIndex=5",
			want:        PaginationParams{StartIndex: 5, Count: DefaultPageSize},
		},
		{
			name:        "custom count",
			queryString: "count=25",
			want:        PaginationParams{StartIndex: 1, Count: 25},
		},
		{
			name:        "both startIndex and count",
			queryString: "startIndex=10&count=50",
			want:        PaginationParams{StartIndex: 10, Count: 50},
		},
		{
			name:        "count clamped to MaxPageSize",
			queryString: "count=5000",
			want:        PaginationParams{StartIndex: 1, Count: MaxPageSize},
		},
		{
			name:        "count=0 is valid",
			queryString: "count=0",
			want:        PaginationParams{StartIndex: 1, Count: 0},
		},
		{
			name:        "invalid startIndex - not a number",
			queryString: "startIndex=abc",
			wantErr:     "invalid startIndex: must be an integer",
		},
		{
			name:        "invalid startIndex - zero",
			queryString: "startIndex=0",
			wantErr:     "invalid startIndex: must be >= 1",
		},
		{
			name:        "invalid startIndex - negative",
			queryString: "startIndex=-1",
			wantErr:     "invalid startIndex: must be >= 1",
		},
		{
			name:        "invalid count - not a number",
			queryString: "count=xyz",
			wantErr:     "invalid count: must be an integer",
		},
		{
			name:        "invalid count - negative",
			queryString: "count=-10",
			wantErr:     "invalid count: must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/users?"+tt.queryString, nil)
			got, err := ParsePaginationParams(req)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPaginate(t *testing.T) {
	// Create test data: items 0-9.
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	tests := []struct {
		name           string
		items          []int
		params         PaginationParams
		wantResult     []int
		wantStartIndex int
	}{
		{
			name:           "first page",
			items:          items,
			params:         PaginationParams{StartIndex: 1, Count: 3},
			wantResult:     []int{0, 1, 2},
			wantStartIndex: 1,
		},
		{
			name:           "second page",
			items:          items,
			params:         PaginationParams{StartIndex: 4, Count: 3},
			wantResult:     []int{3, 4, 5},
			wantStartIndex: 4,
		},
		{
			name:           "last partial page",
			items:          items,
			params:         PaginationParams{StartIndex: 8, Count: 5},
			wantResult:     []int{7, 8, 9},
			wantStartIndex: 8,
		},
		{
			name:           "startIndex beyond total",
			items:          items,
			params:         PaginationParams{StartIndex: 100, Count: 10},
			wantResult:     []int{},
			wantStartIndex: 100,
		},
		{
			name:           "count=0 returns empty",
			items:          items,
			params:         PaginationParams{StartIndex: 1, Count: 0},
			wantResult:     []int{},
			wantStartIndex: 1,
		},
		{
			name:           "empty items",
			items:          []int{},
			params:         PaginationParams{StartIndex: 1, Count: 10},
			wantResult:     []int{},
			wantStartIndex: 1,
		},
		{
			name:           "count larger than total",
			items:          items,
			params:         PaginationParams{StartIndex: 1, Count: 100},
			wantResult:     items,
			wantStartIndex: 1,
		},
		{
			name:           "exact page boundary",
			items:          items,
			params:         PaginationParams{StartIndex: 6, Count: 5},
			wantResult:     []int{5, 6, 7, 8, 9},
			wantStartIndex: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, startIndex := Paginate(tt.items, tt.params)
			assert.Equal(t, tt.wantResult, result)
			assert.Equal(t, tt.wantStartIndex, startIndex)
		})
	}
}

func TestPaginateContinuity(t *testing.T) {
	// Verify that paginating through all pages gives us all items without duplicates or gaps.
	items := make([]int, 25)
	for i := range items {
		items[i] = i
	}

	pageSize := 7
	var collected []int

	for startIndex := 1; startIndex <= len(items); startIndex += pageSize {
		result, _ := Paginate(items, PaginationParams{StartIndex: startIndex, Count: pageSize})
		collected = append(collected, result...)
	}

	assert.Equal(t, items, collected, "pagination should return all items without duplicates or gaps")
}
