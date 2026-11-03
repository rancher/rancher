package common_test

import (
	"maps"
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/rancher/rancher/pkg/auth/providers/saml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				inputMap, ok := tt.input.(map[string]any)
				require.True(t, ok)
				inputMeta, ok := inputMap["metadata"].(map[string]any)
				require.True(t, ok)
				outputConfig, ok := tt.output.(*apimgmtv3.AuthConfig)
				require.True(t, ok)
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

func TestIsValidUserExtraAttribute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want bool
	}{
		{key: common.UserAttributePrincipalID, want: true},
		{key: common.UserAttributeUserName, want: true},
		{key: "PrincipalID", want: true},
		{key: "externalid", want: false},
		{key: "email", want: false},
		{key: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, common.IsValidUserExtraAttribute(tt.key))
		})
	}
}

func TestMergeUserExtraAttributes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		stored map[string][]string
		update map[string][]string
		want   map[string][]string
	}{
		{
			name: "keeps stored keys the update doesn't set",
			stored: map[string][]string{
				common.UserAttributePrincipalID: {"okta_user://alice"},
				common.UserAttributeUserName:    {"alice"},
				"externalid":                    {"00u123"},
				"email":                         {"alice@example.com"},
			},
			update: map[string][]string{
				common.UserAttributePrincipalID: {"okta_user://alice"},
				common.UserAttributeUserName:    {"alice.login"},
			},
			want: map[string][]string{
				common.UserAttributePrincipalID: {"okta_user://alice"},
				common.UserAttributeUserName:    {"alice.login"},
				"externalid":                    {"00u123"},
				"email":                         {"alice@example.com"},
			},
		},
		{
			name:   "nothing stored",
			update: map[string][]string{common.UserAttributeUserName: {"alice"}},
			want:   map[string][]string{common.UserAttributeUserName: {"alice"}},
		},
		{
			name:   "empty update",
			stored: map[string][]string{"email": {"alice@example.com"}},
			want:   map[string][]string{"email": {"alice@example.com"}},
		},
		{
			name: "nothing stored and empty update",
			want: map[string][]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var storedBefore map[string][]string
			if tt.stored != nil {
				storedBefore = maps.Clone(tt.stored)
			}

			got := common.MergeUserExtraAttributes(tt.stored, tt.update)
			assert.Equal(t, tt.want, got)
			// The stored map isn't changed in place.
			assert.Equal(t, storedBefore, tt.stored)
		})
	}
}
