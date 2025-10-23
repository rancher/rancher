package util

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReturnAPIError(t *testing.T) {
	tests := []struct {
		desc     string
		err      error
		code     validation.ErrorCode
		message  string
		response APIErrorResponse
	}{
		{
			desc: "not an API error",
			err:  errors.New("some error"),
			code: validation.ServerError,
		},
		{
			desc: "API error no message",
			err:  apierror.NewAPIError(validation.InvalidBodyContent, ""),
			code: validation.InvalidBodyContent,
		},
		{
			desc:    "API error with message",
			err:     apierror.NewAPIError(validation.Unauthorized, "must authorize"),
			code:    validation.Unauthorized,
			message: "must authorize",
		},
		{
			desc: "norman APIError no message",
			err:  httperror.NewAPIError(httperror.NotFound, ""),
			code: validation.NotFound,
		},
		{
			desc:    "norman APIError with message",
			err:     httperror.NewAPIError(httperror.PermissionDenied, "not allowed"),
			code:    validation.PermissionDenied,
			message: "not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			w := httptest.NewRecorder()
			ReturnAPIError(w, tt.err)

			assert.Equal(t, w.Header().Get("Content-Type"), "application/json")
			assert.Equal(t, tt.code.Status, w.Result().StatusCode)
			response := APIErrorResponse{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Equal(t, tt.code.Status, response.Status)
			assert.Equal(t, tt.code.Code, response.Code)
			assert.Equal(t, tt.message, response.Message)
			assert.Equal(t, "error", response.Type)
			assert.Equal(t, "error", response.BaseType)
		})
	}
}

func TestIsAPIError(t *testing.T) {
	tests := []struct {
		desc       string
		err        error
		isAPIError bool
	}{
		{
			desc: "not an API error",
			err:  errors.New("some error"),
		},
		{
			desc:       "API error",
			err:        apierror.NewAPIError(validation.InvalidBodyContent, ""),
			isAPIError: true,
		},
		{
			desc:       "norman API error",
			err:        httperror.NewAPIError(httperror.NotFound, ""),
			isAPIError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.isAPIError, IsAPIError(tt.err))
		})
	}
}
