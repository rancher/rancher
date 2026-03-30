package publicapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3public"
)

func TestProviderInputForType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerType string
		wantNil      bool
		wantType     string
		wantName     string
	}{
		{
			name:         "canonical Google OAuth type",
			providerType: client.GoogleOAuthProviderType,
			wantType:     client.GoogleOAuthProviderType,
			wantName:     "googleoauth",
		},
		{
			name:         "lowercase Google OAuth variant",
			providerType: "googleOauthProvider",
			wantType:     "googleOauthProvider",
			wantName:     "googleoauth",
		},
		{
			name:         "local provider",
			providerType: client.LocalProviderType,
			wantType:     client.LocalProviderType,
			wantName:     "local",
		},
		{
			name:         "unknown provider returns nil",
			providerType: "unknownProvider",
			wantNil:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerInputForType(tt.providerType)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.wantType, got.GetType())
			assert.Equal(t, tt.wantName, got.GetName())

			if tt.providerType == client.GoogleOAuthProviderType || tt.providerType == "googleOauthProvider" {
				_, ok := got.(*apiv3.GoogleOauthLogin)
				assert.True(t, ok, "expected *apiv3.GoogleOauthLogin")
			}
		})
	}
}
