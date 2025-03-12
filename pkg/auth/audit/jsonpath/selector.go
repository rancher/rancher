package jsonpath

import (
	"reflect"
	"slices"
)

type selector interface {
	// Select checks a path for Matching values and returns how many Path nodes were consumed finding a match. Some selectors like selectChild need more context to understand if a node matches, and therefore need the
	Select(path Path) (int, bool)
}

type SelectorFunc func(Path) (int, bool)

func (f SelectorFunc) Select(p Path) (int, bool) {
	return f(p)
}

type selectRootElement struct{}

func (s selectRootElement) Select(p Path) (int, bool) {
	if len(p) == 0 {
		return 0, false
	}

	if p[0].identifier == nil {
		return 0, false
	}

	if *p[0].identifier != rootNodeBody {
		return 0, false
	}

	return 1, true
}

type subscript struct {
	start *int
	end   *int
	step  *int

	union []int

	isWildcard bool
}

type selectChild struct {
	identifier string
	isWildcard bool
	subscript  *subscript
	union      []string
}

func (c selectChild) matchNodeIdentifier(n node) bool {
	if n.identifier == nil {
		return false
	}

	if c.isWildcard {
		return true
	}

	return *n.identifier == c.identifier
}

func (c selectChild) matchNodeIndexRange(n node) bool {
	switch {
	case n.index == nil:
		return false

	case c.subscript.isWildcard:
		return true

	case len(c.subscript.union) > 0:
		return slices.Contains(c.subscript.union, *n.index)

	case c.subscript.start != nil && c.subscript.end == nil:
		start := *c.subscript.start

		if start < 0 {
			if reflect.ValueOf(n.value).Kind() != reflect.Slice {
				return false
			}

			sliceLen := reflect.ValueOf(n.value).Len()
			start = sliceLen - -start%sliceLen
		}

		return *n.index == start

	case c.subscript.start != nil && c.subscript.end != nil && c.subscript.step != nil:
		if reflect.ValueOf(n.value).Kind() != reflect.Slice {
			return false
		}

		sliceLen := reflect.ValueOf(n.value).Len()

		start := *c.subscript.start
		end := *c.subscript.end
		step := *c.subscript.step

		if start < 0 {
			start = sliceLen - -start%sliceLen
		}

		if end < 0 {
			end = sliceLen - -end%sliceLen + 1
		}

		if end > sliceLen || *n.index < start || *n.index >= end {
			return false
		}

		return (*n.index-start)%step == 0
	}

	panic("bug: this bit should never be reached")
}

func (c selectChild) Select(p Path) (int, bool) {
	if len(p) == 0 {
		return 0, false
	}

	node := p[0]

	if node.identifier == nil {
		return 0, false
	}

	if len(c.union) > 0 {
		if slices.Contains(c.union, *node.identifier) {
			return 1, true
		}

		return 0, false
	}

	if !c.matchNodeIdentifier(node) {
		return 0, false
	}

	if c.subscript == nil {
		return 1, true
	}

	if len(p) < 2 {
		return 0, false
	}

	node = p[1]

	if !c.matchNodeIndexRange(node) {
		return 0, false
	}

	return 2, true
}

type selectRecursiveDescent struct {
	inner selector
}

func (d selectRecursiveDescent) Select(p Path) (int, bool) {
	for i := range p {
		n, ok := d.inner.Select(p[i:])
		if ok {
			return i + n, true
		}
	}

	return 0, false
}
