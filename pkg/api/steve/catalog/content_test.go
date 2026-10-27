package catalog

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/rancher/pkg/catalogv2/git"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetIconHeaders(t *testing.T) {
	tests := []struct {
		name   string
		suffix string
	}{
		{
			name:   ".svg suffix",
			suffix: ".svg",
		},
		{
			name:   ".png suffix",
			suffix: ".png",
		},
		{
			name:   "no suffix",
			suffix: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			setIconHeaders(rw, tt.suffix)
			resp := rw.Result()
			assert.Equal(t, []string{"max-age=31536000, public"}, resp.Header["Cache-Control"])
			assert.Equal(t, []string{"default-src 'none'; style-src 'unsafe-inline'; sandbox"}, resp.Header["Content-Security-Policy"])
			assert.Equal(t, []string{"nosniff"}, resp.Header["X-Content-Type-Options"])
			if tt.suffix == ".svg" {
				assert.Equal(t, []string{"image/svg+xml"}, resp.Header["Content-Type"])
			} else {
				assert.Nil(t, resp.Header["Content-Type"])
			}
		})
	}
}

func TestWriteErrorRetryAfter(t *testing.T) {
	notReady := apierror.WrapAPIError(git.ErrNotCheckedOut, validation.ClusterUnavailable, git.ErrNotCheckedOut.Error())

	tests := []struct {
		name               string
		err                error
		expectedRetryAfter string
	}{
		{name: "repository not checked out yet", err: notReady, expectedRetryAfter: "5"},
		{name: "other 503", err: apierror.NewAPIError(validation.ClusterUnavailable, "unrelated 503")},
		{name: "other error", err: errors.New("boom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			var written error
			apiContext := &types.APIRequest{
				ErrorHandler: func(_ *types.APIRequest, err error) { written = err },
			}

			(&contentDownload{}).writeError(apiContext, rw, tt.err)

			assert.Equal(t, tt.err, written, "the error should still be written")
			assert.Equal(t, tt.expectedRetryAfter, rw.Header().Get("Retry-After"))
		})
	}
}
