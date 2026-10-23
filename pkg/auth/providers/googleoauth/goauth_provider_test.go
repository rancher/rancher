package googleoauth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rancher/norman/httperror"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWrapGoogleNonTransient(t *testing.T) {
	t.Parallel()

	invalidGrant := &oauth2.RetrieveError{ErrorCode: "invalid_grant"}
	wrappedInvalidGrant := fmt.Errorf("token refresh failed: %w", invalidGrant)

	tests := []struct {
		name             string
		err              error
		wantNonTransient bool
	}{
		{
			name:             "Google API 404 error",
			err:              &googleapi.Error{Code: http.StatusNotFound},
			wantNonTransient: true,
		},
		{
			name:             "Google API 403 error",
			err:              &googleapi.Error{Code: http.StatusForbidden},
			wantNonTransient: false,
		},
		{
			name:             "Google API 500 error",
			err:              &googleapi.Error{Code: http.StatusInternalServerError},
			wantNonTransient: false,
		},
		{
			name:             "OAuth2 invalid_grant",
			err:              invalidGrant,
			wantNonTransient: true,
		},
		{
			name:             "OAuth2 invalid_grant wrapped",
			err:              wrappedInvalidGrant,
			wantNonTransient: true,
		},
		{
			name:             "OAuth2 other retrieve error",
			err:              &oauth2.RetrieveError{ErrorCode: "temporarily_unavailable"},
			wantNonTransient: false,
		},
		{
			name:             "non-Google error",
			err:              fmt.Errorf("some transient error"),
			wantNonTransient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := wrapGoogleNonTransient(tt.err)
			if tt.wantNonTransient {
				var nte *common.NonTransientError
				require.ErrorAs(t, got, &nte)
				assert.Equal(t, tt.err, nte.Unwrap())
			} else {
				assert.Equal(t, tt.err, got)
			}
		})
	}
}

func TestGetPrincipalRejectsOtherConfig(t *testing.T) {
	// The provider has no clients configured, so this fails if the principal
	// is looked up.
	provider := &googleOauthProvider{}
	token := &v3.Token{
		UserPrincipal: v3.Principal{
			ObjectMeta: metav1.ObjectMeta{Name: "googleoauth-corp_user://1"},
		},
	}

	for _, principalID := range []string{"googleoauth-it_user://2", "googleoauth-it_group://3", "googleoauth_user://2"} {
		_, err := provider.GetPrincipal(principalID, token)
		assert.True(t, httperror.IsNotFound(err), "expected NotFound for %s, got %v", principalID, err)
	}
}
