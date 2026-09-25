package clusterstats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrossedV122Boundary(t *testing.T) {
	tests := []struct {
		name        string
		knewVersion bool
		oldMinor    int
		newMinor    int
		expected    bool
	}{
		{
			// The regression this guards. A freshly provisioned cluster has no observed version
			// on its first stats sync, so oldMinor is the zero value. Treating that as "was on
			// 1.21 or older" annotated the agent's pod template and rolled it on every new
			// cluster, costing a full pod startup and a tunnel reconnect mid-provisioning.
			name:        "no version observed yet is not an upgrade",
			knewVersion: false,
			oldMinor:    0,
			newMinor:    33,
			expected:    false,
		},
		{
			name:        "genuine upgrade across the boundary",
			knewVersion: true,
			oldMinor:    21,
			newMinor:    22,
			expected:    true,
		},
		{
			name:        "genuine upgrade from well below the boundary",
			knewVersion: true,
			oldMinor:    19,
			newMinor:    33,
			expected:    true,
		},
		{
			name:        "upgrade entirely above the boundary",
			knewVersion: true,
			oldMinor:    30,
			newMinor:    33,
			expected:    false,
		},
		{
			name:        "upgrade entirely below the boundary",
			knewVersion: true,
			oldMinor:    19,
			newMinor:    21,
			expected:    false,
		},
		{
			name:        "no change at the boundary",
			knewVersion: true,
			oldMinor:    22,
			newMinor:    22,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, crossedV122Boundary(tt.knewVersion, tt.oldMinor, tt.newMinor))
		})
	}
}
