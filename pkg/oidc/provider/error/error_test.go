package error

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteError(t *testing.T) {
	const (
		errorMessage     = "message"
		errorDescription = "description"
		errorCode        = 401
	)
	rec := httptest.NewRecorder()

	WriteError(errorMessage, errorDescription, errorCode, rec)

	assert.Equal(t, errorCode, rec.Code)
	assert.JSONEq(t, `{"error":"`+errorMessage+`","error_description":"`+errorDescription+`"}`, strings.TrimSpace(rec.Body.String()))
}

func TestRedirectWithError(t *testing.T) {
	const (
		errorMessage     = "message"
		errorDescription = "description"
		redirectURI      = "http://localhost"
		state            = "state"
	)
	rec := httptest.NewRecorder()

	RedirectWithError(redirectURI, errorMessage, errorDescription, state, rec, &http.Request{})

	assert.Equal(t, 302, rec.Code)
	assert.Equal(t, redirectURI+"?error="+errorMessage+"&error_description="+errorDescription+"&state="+state, rec.Header().Get("Location"))
}
