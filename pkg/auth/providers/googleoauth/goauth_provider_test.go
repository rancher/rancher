package googleoauth

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rancher/rancher/pkg/auth/providers/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"
)

func TestWrapGoogleNonTransient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		err           error
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
