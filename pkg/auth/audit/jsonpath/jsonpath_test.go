package jsonpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestSimpleJSONPath(t *testing.T) {
	jsonPath := JSONPath{
		selectors: []selector{
			selectRootElement{},
			selectRecursiveDescent{
				inner: selectChild{
					identifier: "child",
				},
			},
			selectChild{
				identifier: "grandchild",
			},
		},
	}

	matched := jsonPath.Matches(PathBuilder{}.
		WithRootNode().
		WithChildNode("grandparent").
		WithChildNode("parent").
		WithChildNode("child").
		WithChildNode("grandchild").
		Build())
	assert.True(t, matched)

	matched = jsonPath.Matches(PathBuilder{}.
		WithRootNode().
		WithChildNode("grandparent").
		WithChildNode("parent").
		WithChildNode("child").
		Build())
	assert.False(t, matched)

	matched = jsonPath.Matches(PathBuilder{}.
		WithRootNode().
		WithChildNode("grandparent").
		WithChildNode("parent").
		WithChildNode("grandchild").
		Build())
	assert.False(t, matched)
}

func TestJsonPathSet(t *testing.T) {
	newValue := "[redacted]"
	type testcase struct {
		Name     string
		JSONPath *JSONPath
		Object   map[string]any
		Expected map[string]any
	}

	cases := []testcase{
		{
			Name: "Set Object",
			JSONPath: &JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent"},
					selectChild{identifier: "child"},
				},
			},
			Object: map[string]any{
				"parent": map[string]any{
					"child":   true,
					"sibling": true,
				},
				"other_parent": true,
			},
			Expected: map[string]any{
				"parent": map[string]any{
					"child":   newValue,
					"sibling": true,
				},
				"other_parent": true,
			},
		},
		{
			Name: "Set Array Element",
			JSONPath: &JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", subscript: &subscript{start: ptr.To(1), end: ptr.To(3), step: ptr.To(1)}},
				},
			},
			Object: map[string]any{
				"parent": []any{"zero", "one", "two", "three"},
			},
			Expected: map[string]any{
				"parent": []any{"zero", newValue, newValue, "three"},
			},
		},
		{
			Name: "With Recusive Descent",
			JSONPath: &JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectRecursiveDescent{
						inner: selectChild{identifier: "grandchild"},
					},
				},
			},
			Object: map[string]any{
				"parent": map[string]any{
					"child": map[string]any{
						"grandchild": map[string]any{
							"greatgrandchild": true,
						},
					},
				},
				"child": map[string]any{
					"grandchild": true,
				},
			},
			Expected: map[string]any{
				"parent": map[string]any{
					"child": map[string]any{
						"grandchild": newValue,
					},
				},
				"child": map[string]any{
					"grandchild": newValue,
				},
			},
		},
		{
			Name: "With Wildcard",
			JSONPath: &JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent"},
					selectChild{isWildcard: true},
				},
			},
			Object: map[string]any{
				"parent": map[string]any{
					"child":   true,
					"sibling": true,
				},
			},
			Expected: map[string]any{
				"parent": map[string]any{
					"child":   newValue,
					"sibling": newValue,
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			c.JSONPath.Set(c.Object, newValue)
			assert.Equal(t, c.Expected, c.Object)
		})
	}
}
