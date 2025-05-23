package changes

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestApplyMetricsMerge(t *testing.T) {
	metrics1 := &ApplyMetrics{Create: 1}
	metrics2 := &ApplyMetrics{Create: 1, Delete: 1}

	want := &ApplyMetrics{
		Create: 2,
		Delete: 1,
	}

	if diff := cmp.Diff(want, metrics1.Combine(metrics2)); diff != "" {
		t.Errorf("failed calculate metrics: diff -want +got\n%s", diff)
	}
}
