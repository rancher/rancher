package workload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSchedulingRuleFoundLinux(t *testing.T) {
	type testcase struct {
		name       string
		rule       string
		foundLinux bool
	}
	testcases := []testcase{
		testcase{
			name:       "rule operator equal",
			rule:       "beta.kubernetes.io/os = linux",
			foundLinux: true,
		},
		testcase{
			name:       "rule operator not equal",
			rule:       "beta.kubernetes.io/os != windows",
			foundLinux: true,
		},
		testcase{
			name:       "rule operator in",
			rule:       "beta.kubernetes.io/os in (linux , darwin)",
			foundLinux: true,
		},
		testcase{
			name:       "rule key not match",
			rule:       "test = test2",
			foundLinux: false,
		},
	}
	for _, c := range testcases {
		assert.Equalf(t, c.foundLinux, schedulingRuleFoundLinux([]string{c.rule}), "case %s failed, function schedulingRuleFoundLinux is not as expected", c.name)
	}
}
