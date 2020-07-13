package mapper

import (
	"reflect"
	"testing"
)

var (
	metrics = []map[string]interface{}{
		{"type": "Resource", "resource": "abc"},
		{"type": "Object", "object": "def"},
	}
	currentMetrics = []map[string]interface{}{
		{"type": "Resource", "currentResource": "tuvw"},
		{"type": "Object", "currentObject": "xyz"},
	}
	origin = map[string]interface{}{
		"metrics":        metrics,
		"currentMetrics": currentMetrics,
	}
)

func Test_MergeList(t *testing.T) {
	mapper := NewMergeListByIndexMapper("currentMetrics", "metrics", "type")
	mapper.fromFields = []string{"type", "currentResource", "currentObject"}
	internal := map[string]interface{}{
		"metrics":        metrics,
		"currentMetrics": currentMetrics,
	}
	mapper.FromInternal(internal)

	if err := mapper.ToInternal(internal); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(internal, origin) {
		t.Fatal("merge list not match after parse")
	}
}
