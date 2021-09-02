package catalog

import (
	"net/http/httptest"
	"testing"

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
