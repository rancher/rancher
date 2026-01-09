package oidc

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetCookie(t *testing.T) {
	tests := []struct {
		name           string
		cookieName     string
		cookieValue    string
		expires        time.Time
		requestScheme  string
		expectedSecure bool
	}{
		{
			name:           "https request sets secure cookie",
			cookieName:     "test-cookie",
			cookieValue:    "test-value",
			expires:        time.Now().Add(10 * time.Minute),
			requestScheme:  "https",
			expectedSecure: true,
		},
		{
			name:           "http request sets non-secure cookie",
			cookieName:     "test-cookie",
			cookieValue:    "test-value",
			expires:        time.Now().Add(10 * time.Minute),
			requestScheme:  "http",
			expectedSecure: false,
		},
		{
			name:           "cookie with empty expiry",
			cookieName:     "session-cookie",
			cookieValue:    "session-value",
			expires:        time.Time{},
			requestScheme:  "https",
			expectedSecure: true,
		},
		{
			name:           "cookie with negative expiry for deletion",
			cookieName:     "delete-cookie",
			cookieValue:    "",
			expires:        time.Now().Add(-10 * time.Second),
			requestScheme:  "https",
			expectedSecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestScheme+"://example.com/test", nil)
			w := httptest.NewRecorder()

			setCookie(req, w, tt.cookieName, tt.cookieValue, tt.expires)

			result := w.Result()
			cookies := result.Cookies()
			require.Len(t, cookies, 1, "should set exactly one cookie")

			cookie := cookies[0]
			assert.Equal(t, tt.cookieName, cookie.Name)
			assert.Equal(t, tt.cookieValue, cookie.Value)
			assert.Equal(t, "/", cookie.Path)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, tt.expectedSecure, cookie.Secure)

			if !tt.expires.IsZero() {
				// Allow 1 second tolerance for timing differences
				assert.WithinDuration(t, tt.expires, cookie.Expires, time.Second)
			}
		})
	}
}

func TestGetCookieValue(t *testing.T) {
	tests := []struct {
		name          string
		cookieName    string
		cookies       []*http.Cookie
		expectedValue string
	}{
		{
			name:       "get existing cookie",
			cookieName: "test-cookie",
			cookies: []*http.Cookie{
				{Name: "test-cookie", Value: "test-value"},
			},
			expectedValue: "test-value",
		},
		{
			name:       "get non-existing cookie returns empty string",
			cookieName: "missing-cookie",
			cookies: []*http.Cookie{
				{Name: "other-cookie", Value: "other-value"},
			},
			expectedValue: "",
		},
		{
			name:          "no cookies returns empty string",
			cookieName:    "any-cookie",
			cookies:       []*http.Cookie{},
			expectedValue: "",
		},
		{
			name:       "get specific cookie from multiple cookies",
			cookieName: "target-cookie",
			cookies: []*http.Cookie{
				{Name: "cookie1", Value: "value1"},
				{Name: "target-cookie", Value: "target-value"},
				{Name: "cookie2", Value: "value2"},
			},
			expectedValue: "target-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://example.com/test", nil)
			for _, cookie := range tt.cookies {
				req.AddCookie(cookie)
			}

			value := getCookieValue(req, tt.cookieName)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestSetPKCEVerifier(t *testing.T) {
	tests := []struct {
		name          string
		verifierValue string
		requestScheme string
	}{
		{
			name:          "set PKCE verifier on https",
			verifierValue: "test-verifier-value",
			requestScheme: "https",
		},
		{
			name:          "set PKCE verifier on http",
			verifierValue: "another-verifier",
			requestScheme: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestScheme+"://example.com/test", nil)
			w := httptest.NewRecorder()

			SetPKCEVerifier(req, w, tt.verifierValue)

			result := w.Result()
			cookies := result.Cookies()
			require.Len(t, cookies, 1)

			cookie := cookies[0]
			assert.Equal(t, pkceVerifierCookieName, cookie.Name)
			assert.Equal(t, tt.verifierValue, cookie.Value)
			assert.Equal(t, "/", cookie.Path)
			assert.True(t, cookie.HttpOnly)
			assert.WithinDuration(t, time.Now().Add(10*time.Minute), cookie.Expires, time.Second)
		})
	}
}

func TestGetPKCEVerifier(t *testing.T) {
	tests := []struct {
		name          string
		cookies       []*http.Cookie
		expectedValue string
	}{
		{
			name: "get existing PKCE verifier",
			cookies: []*http.Cookie{
				{Name: pkceVerifierCookieName, Value: "verifier-123"},
			},
			expectedValue: "verifier-123",
		},
		{
			name:          "no PKCE verifier cookie returns empty",
			cookies:       []*http.Cookie{},
			expectedValue: "",
		},
		{
			name: "get PKCE verifier from multiple cookies",
			cookies: []*http.Cookie{
				{Name: "other-cookie", Value: "other"},
				{Name: pkceVerifierCookieName, Value: "my-verifier"},
				{Name: IDTokenCookie, Value: "id-token"},
			},
			expectedValue: "my-verifier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://example.com/test", nil)
			for _, cookie := range tt.cookies {
				req.AddCookie(cookie)
			}

			value := getPKCEVerifier(req)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestDeletePKCEVerifier(t *testing.T) {
	tests := []struct {
		name          string
		requestScheme string
	}{
		{
			name:          "delete PKCE verifier on https",
			requestScheme: "https",
		},
		{
			name:          "delete PKCE verifier on http",
			requestScheme: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestScheme+"://example.com/test", nil)
			w := httptest.NewRecorder()

			deletePKCEVerifier(req, w)

			cookies := w.Result().Cookies()
			require.Len(t, cookies, 1)

			cookie := cookies[0]
			assert.Equal(t, pkceVerifierCookieName, cookie.Name)
			assert.Equal(t, "", cookie.Value)
			assert.True(t, cookie.Expires.Before(time.Now()), "cookie should be expired for deletion")
		})
	}
}

func TestSetIDToken(t *testing.T) {
	tests := []struct {
		name          string
		tokenValue    string
		requestScheme string
	}{
		{
			name:          "set ID token on https",
			tokenValue:    "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...",
			requestScheme: "https",
		},
		{
			name:          "set ID token on http",
			tokenValue:    "test-id-token",
			requestScheme: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.requestScheme+"://example.com/test", nil)
			w := httptest.NewRecorder()

			setIDToken(req, w, tt.tokenValue)

			result := w.Result()
			cookies := result.Cookies()
			require.Len(t, cookies, 1)

			cookie := cookies[0]
			assert.Equal(t, IDTokenCookie, cookie.Name)
			assert.Equal(t, tt.tokenValue, cookie.Value)
			assert.Equal(t, "/", cookie.Path)
			assert.True(t, cookie.HttpOnly)
			// ID token cookie is a session cookie (no expiry set)
			assert.True(t, cookie.Expires.IsZero())
		})
	}
}

func TestGetIDToken(t *testing.T) {
	tests := []struct {
		name          string
		cookies       []*http.Cookie
		expectedValue string
	}{
		{
			name: "get existing ID token",
			cookies: []*http.Cookie{
				{Name: IDTokenCookie, Value: "id-token-value"},
			},
			expectedValue: "id-token-value",
		},
		{
			name:          "no ID token cookie returns empty",
			cookies:       []*http.Cookie{},
			expectedValue: "",
		},
		{
			name: "get ID token from multiple cookies",
			cookies: []*http.Cookie{
				{Name: "session", Value: "session-value"},
				{Name: IDTokenCookie, Value: "my-id-token"},
				{Name: pkceVerifierCookieName, Value: "verifier"},
			},
			expectedValue: "my-id-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "https://example.com/test", nil)
			for _, cookie := range tt.cookies {
				req.AddCookie(cookie)
			}

			value := getIDToken(req)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestCookieRoundTrip(t *testing.T) {
	t.Run("PKCE verifier round trip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/test", nil)
		w := httptest.NewRecorder()
		expectedVerifier := "test-verifier-12345"

		SetPKCEVerifier(req, w, expectedVerifier)

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		newReq := httptest.NewRequest(http.MethodGet, "https://example.com/callback", nil)
		newReq.AddCookie(cookies[0])

		assert.Equal(t, expectedVerifier, getPKCEVerifier(newReq))
	})

	t.Run("ID token round trip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/test", nil)
		w := httptest.NewRecorder()
		expectedToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test.signature"

		setIDToken(req, w, expectedToken)

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		newReq := httptest.NewRequest(http.MethodGet, "https://example.com/callback", nil)
		newReq.AddCookie(cookies[0])

		assert.Equal(t, expectedToken, getIDToken(newReq))
	})

	t.Run("delete PKCE verifier removes it", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "https://example.com/test", nil)
		req.AddCookie(&http.Cookie{Name: pkceVerifierCookieName, Value: "existing-verifier"})

		w := httptest.NewRecorder()
		deletePKCEVerifier(req, w)

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)

		cookie := cookies[0]
		assert.Equal(t, pkceVerifierCookieName, cookie.Name)
		assert.Equal(t, "", cookie.Value)
		assert.True(t, cookie.Expires.Before(time.Now()))
	})
}
