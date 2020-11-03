package aks

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2019-10-01/containerservice"
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

func TestLoadBalancerSKUDefault(t *testing.T) {
	driver := NewDriver()
	flags, err := driver.GetDriverCreateOptions(context.TODO())
	a := assert.New(t)
	a.NoError(err)
	a.Equal(flags.Options["load-balancer-sku"].GetValue(), string(containerservice.Standard))
}
