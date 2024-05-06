package common_test

import (
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestDecode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   any
		output  any
		wantErr bool
	}{
		{
			name:    "successful decode",
			input:   getMockAuthConfig(),
			output:  &apimgmtv3.AuthConfig{},
			wantErr: false,
		},
		{
			name:    "unsuccessful decoder create",
			input:   getMockAuthConfig(),
			output:  apimgmtv3.AuthConfig{},
			wantErr: true,
		},
		{
			name:    "unsuccessful decode",
			input:   "bogus input",
			output:  &apimgmtv3.AuthConfig{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := common.Decode(tt.input, tt.output)
			assert.Equal(t, err != nil, tt.wantErr)
			if !tt.wantErr {
				inputMap, _ := tt.input.(map[string]interface{})
				inputMeta, _ := inputMap["metadata"].(map[string]interface{})
				outputConfig, _ := tt.output.(*apimgmtv3.AuthConfig)
				// Spot check some fields, creationtimestamp is critical though as it's the
				// main reason we are using a customized decoder.
				assert.Equal(t, inputMap["kind"], outputConfig.Kind)
				assert.Equal(t, inputMap["enabled"], outputConfig.Enabled)
				assert.Equal(t, inputMeta["creationtimestamp"], outputConfig.ObjectMeta.CreationTimestamp)
			}
		})
	}
}

func getMockAuthConfig() map[string]any {
	timeStamp, _ := time.Parse(time.RFC3339, "2023-05-15T19:28:22Z")
	createdTime := metav1.NewTime(timeStamp)
	return map[string]any{
		"metadata": map[string]any{
			"name":              saml.ShibbolethName,
			"creationtimestamp": createdTime,
		},
		"kind":       "AuthConfig",
		"apiVersion": "management.cattle.io/v3",
		"type":       "shibbolethConfig",
		"enabled":    true,
		"openLdapConfig": map[string]any{
			"serviceAccountPassword": "testpass1234",
		},
	}
}
