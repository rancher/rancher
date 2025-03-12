package jsonpath

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestParse(t *testing.T) {
	type testcase struct {
		Name     string
		Input    string
		Expected JSONPath
		Err      error
	}

	cases := []testcase{
		{
			Name:  "With Simple Dot Notation",
			Input: "$.parent.child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent"},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Simple Bracket Notation",
			Input: "$['parent']['child']",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent"},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Mixed Notations",
			Input: "$.parent['child'].grandchild",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent"},
					selectChild{identifier: "child"},
					selectChild{identifier: "grandchild"},
				},
			},
		},
		{
			Name:  "With Bracket Notation and Range",
			Input: "$['parent'][0:10].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{start: ptr.To(0), end: ptr.To(10)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Wildcard",
			Input: "$.*.child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "*", isWildcard: true},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Range Start",
			Input: "$.parent[0].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{start: ptr.To(0)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Range Start and Empty End",
			Input: "$.parent[0:].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{start: ptr.To(0), end: ptr.To(-1)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Range Start and End",
			Input: "$.parent[0:10].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{start: ptr.To(0), end: ptr.To(10)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Range Empty Start and End",
			Input: "$.parent[:10].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{start: ptr.To(0), end: ptr.To(10)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Full Range",
			Input: "$.parent[0:10:3].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{start: ptr.To(0), end: ptr.To(10), step: ptr.To(3)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Full Unspecified Range",
			Input: "$.parent[::].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{start: ptr.To(0), end: ptr.To(-1), step: ptr.To(1)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Wildcard Range",
			Input: "$.parent[*].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{isWildcard: true}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Recusrive Descent",
			Input: "$.grandparent..child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "grandparent"},
					selectRecursiveDescent{inner: selectChild{identifier: "child"}},
				},
			},
		},
		{
			Name:  "Catch All",
			Input: "$..*",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectRecursiveDescent{inner: selectChild{identifier: "*", isWildcard: true}},
				},
			},
		},
		{
			Name:  "With Quoted Bracket Identifier",
			Input: "$['a \\'quoted\\' field']",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "a 'quoted' field"},
				},
			},
		},
		{
			Name:  "A Bracketed Identifier",
			Input: "$['A [bracketed] string']",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "A [bracketed] string"},
				},
			},
		},
		{
			Name:  "With Simple Identifier Union",
			Input: "$.[a,b,c].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{union: []string{"a", "b", "c"}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Index Union",
			Input: "$.parent[2,4].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{identifier: "parent", r: &indexRange{union: []int{2, 4}}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Union and Bracket Notation",
			Input: "$.[a,b,c]['child']",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{union: []string{"a", "b", "c"}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Union and Range",
			Input: "$.[a,b,c][0:10].child",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectChild{union: []string{"a", "b", "c"}, r: &indexRange{start: ptr.To(0), end: ptr.To(10)}},
					selectChild{identifier: "child"},
				},
			},
		},
		{
			Name:  "With Recursive Descent and Union",
			Input: "$..[a,b,c]",
			Expected: JSONPath{
				selectors: []selector{
					selectRootElement{},
					selectRecursiveDescent{inner: selectChild{union: []string{"a", "b", "c"}}},
				},
			},
		},
		{
			Name:  "With Empty Identifier Union",
			Input: "$.[].child",
			Err:   fmt.Errorf("failed to parse child selector: expected at least one union option"),
		},
		{
			Name:  "With Non Alphabetic Option",
			Input: "$.[?,!].child",
			Err:   fmt.Errorf("failed to parse child selector: only characters in range A-Za-z_- are allowed in union options but found '?'"),
		},
		{
			Name:  "With Missing Dot",
			Input: "$parent.child",
			Err:   fmt.Errorf("unexpected character 'p'"),
		},
		{
			Name:  "With Invalid Dot Notation Field Name",
			Input: "$.with?",
			Err:   fmt.Errorf("failed to parse child selector: only characters in range A-Za-z_- are allowed in dot notation identifiers but found '?'"),
		},
		{
			Name:  "With Unescapped Wildcard",
			Input: "$.with*wildcard.child",
			Err:   fmt.Errorf("failed to parse child selector: found unescaped '*' in identifier"),
		},
		{
			Name:  "With Unescapped Open Bracket",
			Input: "$.with[bracket.child",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: failed to parse range start: failed to parse integer: strconv.Atoi: parsing \"bracket.chil\": invalid syntax"),
		},
		{
			Name:  "With Invalid Index",
			Input: "$.parent[a].child",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: failed to parse range start: failed to parse integer: strconv.Atoi: parsing \"a\": invalid syntax"),
		},
		{
			Name:  "With Invalid Index Range End",
			Input: "$.parent[:a].child",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: failed to parse index range: failed to parse range end: failed to parse integer: strconv.Atoi: parsing \"a\": invalid syntax"),
		},
		{
			Name:  "With Invalid Index Range Step",
			Input: "$.parent[::a].child",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: failed to parse index range: failed to parse range step: failed to parse integer: strconv.Atoi: parsing \"a\": invalid syntax"),
		},
		{
			Name:  "With Unfullfilled Backslash in Bracket",
			Input: "$['with\\",
			Err:   fmt.Errorf("failed to parse child selector: unexpected end of path"),
		},
		{
			Name:  "With Missing Open Single Quote",
			Input: "$[parent']",
			Err:   fmt.Errorf("unexpected character '['"),
		},
		{
			Name:  "With Missing Closing Single Quote",
			Input: "$['parent]",
			Err:   fmt.Errorf("failed to parse child selector: expected \"']\" but none was found"),
		},
		{
			Name:  "With Missing Open Bracket",
			Input: "$'parent']",
			Err:   fmt.Errorf("unexpected character '''"),
		},
		{
			Name:  "With Unescapped Quote",
			Input: "$['bad ' quote']",
			Err:   fmt.Errorf("failed to parse child selector: single quotes must be escapped in bracket notation"),
		},
		{
			Name:  "With Missing Closing Bracket",
			Input: "$['parent'",
			Err:   fmt.Errorf("failed to parse child selector: single quotes must be escapped in bracket notation"),
		},
		{
			Name:  "With Missing End Bracket in Wildcard Index Range",
			Input: "$.parent[*",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: expected \"]\" but none"),
		},
		{
			Name:  "With Missing End Bracket at JSONPath End",
			Input: "$.parent[",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: expected ']' but none"),
		},
		{
			Name:  "With Extra Number in Index Range",
			Input: "$.parent[0:10:3:4].child",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: failed to parse index range: expected \"]\" but found :"),
		},
		{
			Name:  "With No Int After Comma in Index Union",
			Input: "$.parent[0,].child",
			Err:   fmt.Errorf("failed to parse child selector: failed to parse subscript: failed to parse index union: index unions options may not be empty"),
		},
		{
			Name:  "With No Chars after Comma in Union",
			Input: "$.[a,,].child",
			Err:   fmt.Errorf("failed to parse child selector: union options may not be empty"),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			actual, err := Parse(c.Input)
			if c.Err == nil && err != nil {
				t.Fatalf("unexpected error from Parse: %s", err)
			} else if c.Err != nil && err == nil {
				t.Fatalf("expected error '%s' from parse but found <nil>", c.Err)
			} else if c.Err != nil && err != nil {
				assert.Equal(t, c.Err, err)
			}

			if c.Err != nil {
				assert.Nil(t, actual)
			} else {
				assert.Equal(t, c.Expected, *actual)
			}
		})
	}
}
