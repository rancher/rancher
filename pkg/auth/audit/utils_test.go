package audit

import (
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestMergePolicyVerbosities(t *testing.T) {
	type testCase struct {
		Name     string
		Lhs      auditlogv1.LogVerbosity
		Rhs      auditlogv1.LogVerbosity
		Expected auditlogv1.LogVerbosity
	}

	testCases := []testCase{
		{
			Name: "Zeroed Verbosities Returns Zeroed Verbosity",
		},
		{
			Name:     "Single Policy Returns Policy Verbosity",
			Rhs:      verbosityForLevel(auditlogv1.LevelMetadata),
			Expected: verbosityForLevel(auditlogv1.LevelMetadata),
		},
		{
			Name: "Multiple Policies Merge Verbosities",
			Lhs: auditlogv1.LogVerbosity{
				Request: auditlogv1.Verbosity{
					Headers: true,
				},
				Response: auditlogv1.Verbosity{
					Headers: true,
				},
			},
			Rhs: auditlogv1.LogVerbosity{
				Response: auditlogv1.Verbosity{
					Body: true,
				},
			},
			Expected: auditlogv1.LogVerbosity{
				Request: auditlogv1.Verbosity{
					Headers: true,
				},
				Response: auditlogv1.Verbosity{
					Headers: true,
					Body:    true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := mergeLogVerbosities(tc.Lhs, tc.Rhs)
			assert.Equal(t, tc.Expected, actual)
		})
	}
}
