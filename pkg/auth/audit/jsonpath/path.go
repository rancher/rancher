package jsonpath

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/utils/ptr"
)

const (
	rootNodeBody = "$"
)

type node struct {
	identifier *string
	index      *int

	value any
}

// path represents a real path to a field in a JSON object.
type Path []node

func (p Path) String() string {
	if len(p) == 0 {
		return rootNodeBody
	}

	var builder strings.Builder
	for _, n := range p {
		if n.identifier != nil {
			if *n.identifier == rootNodeBody {
				builder.WriteString(rootNodeBody)
			} else {
				builder.WriteString(fmt.Sprintf("['%s']", *n.identifier))
			}
		} else if n.index != nil {
			builder.WriteString(fmt.Sprintf("[%d]", *n.index))
		}
	}

	return builder.String()
}

type PathBuilder struct {
	inner Path
}

func (b PathBuilder) WithRootNode() PathBuilder {
	body := rootNodeBody

	return PathBuilder{
		inner: append(
			slices.Clone(b.inner),
			node{
				identifier: &body,
			},
		),
	}
}

func (b PathBuilder) WithChildNode(name string) PathBuilder {
	return PathBuilder{
		inner: append(
			slices.Clone(b.inner),
			node{
				identifier: &name,
			},
		),
	}
}

func (b PathBuilder) WithIndexNode(index uint, value any) PathBuilder {
	return PathBuilder{
		inner: append(
			slices.Clone(b.inner),
			node{
				index: ptr.To(int(index)),
				value: value,
			},
		),
	}
}

func (b PathBuilder) Build() Path {
	return b.inner
}
