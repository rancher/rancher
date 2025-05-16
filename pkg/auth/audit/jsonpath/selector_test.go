package jsonpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestSelectRootElement(t *testing.T) {
	type testcase struct {
		Name     string
		Path     Path
		Expected bool
	}

	cases := []testcase{
		{
			Name: "Empty Path",
			Path: Path{},
		},
		{
			Name:     "Only Root Node",
			Path:     PathBuilder{}.WithRootNode().Build(),
			Expected: true,
		},
		{
			Name:     "Path with Several Nodes",
			Path:     PathBuilder{}.WithRootNode().WithChildNode("parent").WithChildNode("child").Build(),
			Expected: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			n, ok := selectRootElement{}.Select(c.Path)
			assert.Equal(t, c.Expected, ok)

			if c.Expected {
				assert.Equal(t, 1, n)
			} else {
				assert.Equal(t, 0, n)
			}
		})
	}
}

func TestSelectChild(t *testing.T) {
	type testcase struct {
		Name     string
		Selector selector
		Path     Path
		Expected bool
		Consumed int
	}

	cases := []testcase{
		{
			Name:     "Simple Selector Empty Path",
			Selector: selectChild{identifier: "child"},
		},
		{
			Name:     "Simple Selector Root Only",
			Selector: selectChild{identifier: "child"},
			Path:     PathBuilder{}.WithRootNode().Build(),
		},
		{
			Name:     "Simple Selector Non-Matching Child",
			Selector: selectChild{identifier: "child"},
			Path:     PathBuilder{}.WithChildNode("parent").Build(),
		},
		{
			Name:     "Simple Selector Matching Child Too Deep",
			Selector: selectChild{identifier: "child"},
			Path:     PathBuilder{}.WithRootNode().WithChildNode("parent").WithChildNode("child").Build(),
		},
		{
			Name:     "Simple Selector Matching Child",
			Selector: selectChild{identifier: "child"},
			Path:     PathBuilder{}.WithChildNode("child").Build(),
			Expected: true,
			Consumed: 1,
		},
		{
			Name: "Simple Index Range Selector No Match [0]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(1),
				},
			},
			Path: PathBuilder{}.WithChildNode("child").WithIndexNode(0, make([]string, 10)).Build(),
		},
		{
			Name: "Simple Index Range Selector Match [0]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(0, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector No Match [0:3]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
					end:   ptr.To(3),
					step:  ptr.To(1),
				},
			},
			Path: PathBuilder{}.WithChildNode("child").WithIndexNode(3, make([]string, 10)).Build(),
		},
		{
			Name: "Index Range Selector Match End of [0:3]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
					end:   ptr.To(3),
					step:  ptr.To(1),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(2, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector Match Start of [0:3]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
					end:   ptr.To(3),
					step:  ptr.To(1),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(0, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector [-1]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(-1),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(9, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector [-2]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(-2),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(8, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector [:-1]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
					end:   ptr.To(-1),
					step:  ptr.To(1),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(9, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector 7 Matches [-3:-1]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(-3),
					end:   ptr.To(-1),
					step:  ptr.To(1),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(7, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector Start With Step [:-1:2]",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
					end:   ptr.To(-1),
					step:  ptr.To(2),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(2, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Wildcard Index Range Selector Match 100",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					isWildcard: true,
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(100, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Index Range Selector No Match Stepped Index",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
					end:   ptr.To(10),
					step:  ptr.To(2),
				},
			},
			Path: PathBuilder{}.WithChildNode("child").WithIndexNode(3, make([]string, 10)).Build(),
		},
		{
			Name: "Index Range Selector Match Stepped Index",
			Selector: selectChild{
				identifier: "child",
				subscript: &subscript{
					start: ptr.To(0),
					end:   ptr.To(10),
					step:  ptr.To(2),
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(4, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Wildcard Identifier Match Child",
			Selector: selectChild{
				isWildcard: true,
			},
			Path:     PathBuilder{}.WithChildNode("child").Build(),
			Expected: true,
			Consumed: 1,
		},
		{
			Name: "Wildcard Identifier Match Parent",
			Selector: selectChild{
				isWildcard: true,
			},
			Path:     PathBuilder{}.WithChildNode("Parent").Build(),
			Expected: true,
			Consumed: 1,
		},
		{
			Name: "Wildcard Identifier Wildcard Index Range Selector Match 100",
			Selector: selectChild{
				isWildcard: true,
				subscript: &subscript{
					isWildcard: true,
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithIndexNode(100, make([]string, 10)).Build(),
			Expected: true,
			Consumed: 2,
		},
		{
			Name: "Union Selector No Match",
			Selector: selectChild{
				union: []string{"grandchild", "greatgrandchild"},
			},
			Path: PathBuilder{}.WithChildNode("parent").WithChildNode("child").Build(),
		},
		{
			Name: "Union Selector Has Match",
			Selector: selectChild{
				union: []string{"child", "parent"},
			},
			Path:     PathBuilder{}.WithChildNode("parent").Build(),
			Expected: true,
			Consumed: 1,
		},
		{
			Name: "Index Union No Match",
			Selector: selectChild{
				subscript: &subscript{
					union: []int{2, 4},
				},
			},
			Path: PathBuilder{}.WithChildNode("child").WithIndexNode(1, make([]string, 10)).Build(),
		},
		{
			Name: "Index Union Has Match",
			Selector: selectChild{
				subscript: &subscript{
					union: []int{2, 4},
				},
			},
			Path: PathBuilder{}.WithChildNode("child").WithIndexNode(2, make([]string, 10)).Build(),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			n, ok := c.Selector.Select(c.Path)
			assert.Equal(t, c.Consumed, n)
			assert.Equal(t, c.Expected, ok)
		})
	}
}

func TestRecursiveDescent(t *testing.T) {
	type testcase struct {
		Name     string
		Selector selector
		Path     Path
		Expected bool
		Consumed int
	}

	cases := []testcase{
		{
			Name: "No Select Empty Path",
			Selector: selectRecursiveDescent{
				inner: selectChild{
					identifier: "no-exist",
				},
			},
			Path: PathBuilder{}.WithRootNode().WithChildNode("parent").WithChildNode("child").WithChildNode("grandchild").WithChildNode("greatgrandchild").Build(),
		},
		{
			Name: "No Select Long Path",
			Selector: selectRecursiveDescent{
				inner: selectChild{
					identifier: "child",
				},
			},
			Path: PathBuilder{}.Build(),
		},
		{
			Name: "Selects First Node",
			Selector: selectRecursiveDescent{
				inner: selectChild{
					identifier: "child",
				},
			},
			Path:     PathBuilder{}.WithChildNode("child").WithChildNode("grandchild").Build(),
			Expected: true,
			Consumed: 1,
		},
		{
			Name: "Selects Deep Node",
			Selector: selectRecursiveDescent{
				inner: selectChild{
					identifier: "greatgrandchild",
				},
			},
			Path:     PathBuilder{}.WithRootNode().WithChildNode("parent").WithChildNode("child").WithChildNode("grandchild").WithChildNode("greatgrandchild").Build(),
			Expected: true,
			Consumed: 5,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			n, ok := c.Selector.Select(c.Path)
			assert.Equal(t, c.Consumed, n)
			assert.Equal(t, c.Expected, ok)
		})
	}
}
