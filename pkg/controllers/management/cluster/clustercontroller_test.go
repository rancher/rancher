package cluster

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rancher/norman/types"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

const testServiceNodePortRange = "10000-32769"

type capabilitiesTestCase struct {
	annotations  map[string]string
	capabilities v32.Capabilities
	result       v32.Capabilities
	errMsg       string
}

func TestOverrideCapabilities(t *testing.T) {
	assert := assert.New(t)

	fakeCapabilitiesSchema := types.Schema{
		ResourceFields: map[string]types.Field{
			"pspEnabled": {
				Type: "boolean",
			},
			"nodePortRange": {
				Type: "string",
			},
			"ingressCapabilities": {
				Type: "something",
			},
		},
	}
	tests := []capabilitiesTestCase{
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "pspEnabled"): "true",
			},
			capabilities: v32.Capabilities{},
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "nodePortRange"): "9999",
			},
			capabilities: v32.Capabilities{},
			result: v32.Capabilities{
				NodePortRange: "9999",
			},
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "ingressCapabilities"): "[{\"customDefaultBackend\":true,\"ingressProvider\":\"asdf\"}]",
			},
			capabilities: v32.Capabilities{},
			result: v32.Capabilities{
				IngressCapabilities: []v32.IngressCapabilities{
					{
						CustomDefaultBackend: pointer.BoolPtr(true),
						IngressProvider:      "asdf",
					},
				},
			},
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "notarealcapability"): "something",
			},
			capabilities: v32.Capabilities{},
			errMsg:       "resource field [notarealcapability] from capabillities annotation not found",
		},
		{
			annotations: map[string]string{
				fmt.Sprintf("%s%s", capabilitiesAnnotation, "pspEnabled"): "5",
			},
			capabilities: v32.Capabilities{},
			errMsg:       "strconv.ParseBool: parsing \"5\": invalid syntax",
		},
	}

	c := controller{
		capabilitiesSchema: &fakeCapabilitiesSchema,
	}
	for _, test := range tests {
		result, err := c.overrideCapabilities(test.annotations, test.capabilities)
		if err != nil {
			assert.Equal(test.errMsg, err.Error())
		} else {
			assert.True(reflect.DeepEqual(test.result, result))
		}
	}

	assert.Nil(nil)
}
