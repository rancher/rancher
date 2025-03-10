package jsonpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
