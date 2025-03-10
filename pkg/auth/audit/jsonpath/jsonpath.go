package jsonpath

// JSONPath represents a JSONPath expression as described in https://goessner.net/articles/JsonPath. While mostly a
// complete implementation, it does not support all features of the JSONPath. Here is a list of known features not yet supported:
// 1. script expression and filters (ie '(...)' and '?(...)').
// 2. Negative indexing (-1 points to the last element, -2 to the second last, and so on).
// 3. Union operator (',').
type JSONPath struct {
	selectors []selector
}

func (j *JSONPath) Matches(path Path) bool {
	for _, s := range j.selectors {
		i, ok := s.Select(path)
		if !ok {
			return false
		}

		path = path[i:]
	}

	return true
}

func Set(path JSONPath, value any) bool {
	panic("todo: not yet implemented")
}
