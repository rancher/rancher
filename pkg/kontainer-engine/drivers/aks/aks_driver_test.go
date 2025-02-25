package aks

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test that the default LoadBalancerSKU is an empty string. We need this to maintain compatibility
// with older AKS clusters without the LoadBalancerSKU field set upon creation.
func TestLoadBalancerSKUDefault(t *testing.T) {
	driver := NewDriver()
	flags, err := driver.GetDriverCreateOptions(context.TODO())
	a := assert.New(t)
	a.NoError(err)
	a.Equal(flags.Options["load-balancer-sku"].GetValue(), "")
}
