package jsonpath

// JSONPath represents a JSONPath expression as described in https://goessner.net/articles/JsonPath. While mostly a
// complete implementation, it does not support all features of the JSONPath. Here is a list of known features not yet supported:
// 1. script expression and filters (ie '(...)' and '?(...)').
type JSONPath struct {
	selectors []selector
}

func (j *JSONPath) Matches(path Path) bool {
	if len(j.selectors) == 0 {
		return false
	}

	for _, s := range j.selectors {
		i, ok := s.Select(path)
		if !ok {
			return false
		}

		path = path[i:]
	}

	return true
}

func (j *JSONPath) doSetSlice(path PathBuilder, obj []any, value any) {
	for i, v := range obj {
		path := path.WithIndexNode(uint(i), obj)

		if j.Matches(path.Build()) {
			obj[i] = value
			continue
		}

		switch v := v.(type) {
		case map[string]any:
			j.doSetMap(path, v, value)
		case []any:
			j.doSetSlice(path, v, value)
		}
	}
}

func (j *JSONPath) doSetMap(path PathBuilder, obj map[string]any, value any) {
	for k, v := range obj {
		path := path.WithChildNode(k)

		if j.Matches(path.Build()) {
			obj[k] = value
			continue
		}

		switch v := v.(type) {
		case map[string]any:
			j.doSetMap(path, v, value)
		case []any:
			j.doSetSlice(path, v, value)
		}
	}
}

func (j *JSONPath) Set(obj map[string]any, value any) {
	path := PathBuilder{}.WithRootNode()
	j.doSetMap(path, obj, value)
}
