package csp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServeHTTP(t *testing.T) {
	const valid = `{"csp-report":{"document-uri":"https://rancher.test/dashboard/home","blocked-uri":"inline","effective-directive":"script-src-elem","violated-directive":"script-src-elem"}}`

	tests := []struct {
		name     string
		method   string
		body     string
		wantCode int
	}{
		{
			name:     "valid report",
			method:   http.MethodPost,
			body:     valid,
			wantCode: http.StatusNoContent,
		},
		{
			name:     "falls back to violated-directive",
			method:   http.MethodPost,
			body:     `{"csp-report":{"violated-directive":"style-src"}}`,
			wantCode: http.StatusNoContent,
		},
		{
			name:     "get is rejected",
			method:   http.MethodGet,
			body:     valid,
			wantCode: http.StatusMethodNotAllowed,
		},
		{
			name:     "malformed body is rejected",
			method:   http.MethodPost,
			body:     "not json",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "report without a directive is rejected",
			method:   http.MethodPost,
			body:     `{"csp-report":{"blocked-uri":"inline"}}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "oversized body is rejected",
			method:   http.MethodPost,
			body:     `{"csp-report":{"blocked-uri":"` + strings.Repeat("a", maxBodySize) + `"}}`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mux := http.NewServeMux()
			Register(mux)

			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(test.method, ReportPath, strings.NewReader(test.body)))

			assert.Equal(t, test.wantCode, recorder.Code)
		})
	}
}

func TestShouldLog(t *testing.T) {
	h := &handler{seen: map[string]time.Time{}}
	now := time.Now()

	assert.True(t, h.shouldLog("script-src", now))
	assert.False(t, h.shouldLog("script-src", now.Add(logInterval/2)))
	assert.True(t, h.shouldLog("style-src", now))
	assert.True(t, h.shouldLog("script-src", now.Add(logInterval)))
}

func TestShouldLogBoundsTheTable(t *testing.T) {
	h := &handler{seen: map[string]time.Time{}}
	now := time.Now()

	for i := 0; i < maxTrackedViolations*2; i++ {
		require.True(t, h.shouldLog(strings.Repeat("x", i), now))
	}

	assert.LessOrEqual(t, len(h.seen), maxTrackedViolations)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "short", truncate("short"))
	assert.Equal(t, strings.Repeat("a", maxFieldLength)+"...", truncate(strings.Repeat("a", maxFieldLength+1)))
}
