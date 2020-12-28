package aks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_generateUniqueLogWorkspace(t *testing.T) {
	tests := []struct {
		name          string
		workspaceName string
		want          string
	}{
		{
			name:          "basic test",
			workspaceName: "ThisIsAValidInputklasjdfkljasjgireqahtawjsfklakjghrehtuirqhjfhwjkdfhjkawhfdjkhafvjkahg",
			want:          "ThisIsAValidInputklasjdfkljasjgireqahtawjsfkla-fb8fb22278d8eb98",
		},
	}
	for _, tt := range tests {
		got := generateUniqueLogWorkspace(tt.workspaceName)
		assert.Equal(t, tt.want, got)
		assert.Len(t, got, 63)
	}
}

// Test that the default LoadBalancerSKU is an empty string. We need this to maintain compatibility
// with older AKS clusters without the LoadBalancerSKU field set upon creation.
func TestLoadBalancerSKUDefault(t *testing.T) {
	driver := NewDriver()
	flags, err := driver.GetDriverCreateOptions(context.TODO())
	a := assert.New(t)
	a.NoError(err)
	a.Equal(flags.Options["load-balancer-sku"].GetValue(), "")
}
