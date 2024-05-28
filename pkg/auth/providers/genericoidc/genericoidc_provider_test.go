package genericoidc

import (
	"reflect"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/oidc"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenOIDCProvider_GetPrincipal(t *testing.T) {
	tests := []struct {
		name        string
		principalID string
		token       v3.Token
		want        v3.Principal
		wantErr     bool
	}{
		{
			name:        "fetch principal for current user",
			principalID: "genericoidc_user://1234567",
			token: v3.Token{
				UserPrincipal: apimgmtv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: "genericoidc_user://1234567",
					},
					DisplayName:   "Test User",
					LoginName:     "1234567",
					PrincipalType: "user",
					Me:            true,
				},
			},
			want: v3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://1234567",
				},
				DisplayName:   "Test User",
				LoginName:     "1234567",
				PrincipalType: "user",
				Me:            true,
				Provider:      Name,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal for user other than self",
			principalID: "genericoidc_user://9876543",
			token: v3.Token{
				UserPrincipal: apimgmtv3.Principal{
					ObjectMeta: metav1.ObjectMeta{
						Name: "genericoidc_user://1234567",
					},
					DisplayName:   "Test User",
					LoginName:     "1234567",
					PrincipalType: "user",
					Me:            false,
				},
			},
			want: v3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://9876543",
				},
				DisplayName:   "9876543",
				LoginName:     "9876543",
				PrincipalType: "user",
				Me:            false,
				Provider:      Name,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal token is nil",
			principalID: "genericoidc_user://9876543",
			want: v3.Principal{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "genericoidc_user://9876543",
				},
				DisplayName:   "9876543",
				LoginName:     "9876543",
				PrincipalType: "user",
				Me:            false,
				Provider:      Name,
			},
			wantErr: false,
		},
		{
			name:        "fetch principal called with empty principal",
			principalID: "",
			want:        v3.Principal{},
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		g := &GenOIDCProvider{
			oidc.OpenIDCProvider{
				Name: Name,
				Type: client.GenericOIDCConfigType,
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.GetPrincipal(tt.principalID, tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPrincipal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetPrincipal() got = %v, want %v", got, tt.want)
			}
		})
	}
}
