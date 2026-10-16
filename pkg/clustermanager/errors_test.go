package clustermanager_test

import (
	"fmt"
	"testing"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/stretchr/testify/require"
)

func TestIsClusterUnavailableErr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "basic error",
			err:  fmt.Errorf("standard error"),
			want: false,
		},
		{
			name: "api error bad error code",
			err: &httperror.APIError{
				Code: httperror.ActionNotAvailable,
			},
			want: false,
		},
		{
			name: "api error cluster unavailable code",
			err: &httperror.APIError{
				Code: httperror.ClusterUnavailable,
			},
			want: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := clustermanager.IsClusterUnavailableErr(test.err)
			require.Equal(t, got, test.want)
		})
	}
}
