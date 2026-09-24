package common

import (
	"strings"
	"testing"

	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSplitPrincipalID(t *testing.T) {
	tests := []struct {
		name              string
		principalID       string
		wantProviderType  string
		wantExternalID    string
		wantPrincipalType string
		wantErr           bool
	}{
		{
			name:              "valid user principal id",
			principalID:       "github_user://9253000",
			wantProviderType:  "github",
			wantExternalID:    "9253000",
			wantPrincipalType: "user",
		},
		{
			name:              "valid team principal id",
			principalID:       "github_team://9933605",
			wantProviderType:  "github",
			wantExternalID:    "9933605",
			wantPrincipalType: "team",
		},
		{
			name:              "valid principal id without double slash",
			principalID:       "github_org:9343010",
			wantProviderType:  "github",
			wantExternalID:    "9343010",
			wantPrincipalType: "org",
		},
		{
			name:        "invalid principal id missing colon",
			principalID: "github_user//9253000",
			wantErr:     true,
		},
		{
			name:        "invalid principal id missing underscore",
			principalID: "github://9253000",
			wantErr:     true,
		},
		{
			name:              "empty external id is accepted",
			principalID:       "github_user:",
			wantProviderType:  "github",
			wantExternalID:    "",
			wantPrincipalType: "user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProviderType, gotPrincipalType, gotExternalID, err := SplitPrincipalID(tt.principalID)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for principalID %q, got nil", tt.principalID)
				}
				if !strings.Contains(err.Error(), "invalid principal id") {
					t.Fatalf("expected invalid principal id error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error for principalID %q, got %v", tt.principalID, err)
			}

			if gotProviderType != tt.wantProviderType {
				t.Fatalf("expected providerType %q, got %q", tt.wantProviderType, gotProviderType)
			}

			if gotExternalID != tt.wantExternalID {
				t.Fatalf("expected externalID %q, got %q", tt.wantExternalID, gotExternalID)
			}

			if gotPrincipalType != tt.wantPrincipalType {
				t.Fatalf("expected principalType %q, got %q", tt.wantPrincipalType, gotPrincipalType)
			}
		})
	}
}

func TestConfigNameFromToken(t *testing.T) {
	tests := []struct {
		name           string
		principalName  string
		wantConfigName string
		wantErr        bool
	}{
		{
			name:           "github user token",
			principalName:  "github_user://9253000",
			wantConfigName: "github",
		},
		{
			name:           "local user token",
			principalName:  "local_user://admin",
			wantConfigName: "local",
		},
		{
			name:          "invalid principal missing colon",
			principalName: "github_user//9253000",
			wantErr:       true,
		},
		{
			name:          "invalid principal missing underscore",
			principalName: "github://9253000",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &apiv3.Token{
				UserPrincipal: apiv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: tt.principalName,
					},
					LoginName:     "developer",
					PrincipalType: "user",
				},
			}

			gotConfigName, err := ConfigNameFromToken(token)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for principal %q, got nil", tt.principalName)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error for principal %q, got %v", tt.principalName, err)
			}

			if gotConfigName != tt.wantConfigName {
				t.Fatalf("expected config name %q, got %q", tt.wantConfigName, gotConfigName)
			}
		})
	}
}
