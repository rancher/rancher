package jsonpath

import "reflect"

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

type indexRange struct {
	start *int
	end   *int
	step  *int

	isWildcard bool
}

// todo: what happesn for $.*[*]
type selectChild struct {
	identifier string

	isWildcard bool

	r *indexRange
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

	case c.r.isWildcard:
		return true

	case c.r.start != nil && c.r.end == nil:
		return *n.index == *c.r.start

	case c.r.start != nil && c.r.end != nil && c.r.step != nil:
		if reflect.ValueOf(n.value).Kind() != reflect.Slice {
			return false
		}

		sliceLen := reflect.ValueOf(n.value).Len()

		start := *c.r.start
		end := *c.r.end
		step := *c.r.step

		if start < 0 {
			start = sliceLen - -start%sliceLen
		}

		if end < 0 {
			end = sliceLen - -end%sliceLen
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

	if !c.matchNodeIdentifier(node) {
		return 0, false
	}

	if c.r == nil {
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
