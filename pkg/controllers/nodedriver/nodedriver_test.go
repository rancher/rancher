package nodedriver

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
)

func TestOnChange(t *testing.T) {
	tests := []struct {
		name      string
		driver    *v3.NodeDriver
		expected  *v3.NodeDriver
		expectErr bool
	}{
		{
			name:      "nil",
			driver:    nil,
			expected:  nil,
			expectErr: false,
		},
		{
			name:      "not active",
			driver:    &v3.NodeDriver{},
			expected:  &v3.NodeDriver{},
			expectErr: false,
		},
		{
			name:      "builtin",
			driver:    &v3.NodeDriver{Spec: v3.NodeDriverSpec{Builtin: true}},
			expected:  &v3.NodeDriver{Spec: v3.NodeDriverSpec{Builtin: true}},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := onChange("", tt.driver)
			if tt.expectErr {
				assert.NotNil(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
