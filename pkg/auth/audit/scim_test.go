package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureSCIMRequestWithoutAuditLog(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPatch, "/v1-scim/okta/Users/u-abc", strings.NewReader(`{}`))
	body := req.Body

	CaptureSCIMRequest(req)

	assert.True(t, body == req.Body, "the body must not be wrapped")
}

func TestCaptureSCIMRequest(t *testing.T) {
	t.Parallel()

	const (
		reqBody = `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false}]}`
		resBody = `{"id":"u-abc","active":false}`
	)

	tests := []struct {
		name        string
		level       auditlogv1.Level
		method      string
		contentType string
		mark        bool
		read        bool
		wantReqBody bool
		wantResBody bool
	}{
		{name: "PATCH", level: auditlogv1.LevelRequest, method: http.MethodPatch, contentType: contentTypeSCIMJSON, mark: true, read: true, wantReqBody: true},
		{name: "PUT", level: auditlogv1.LevelRequest, method: http.MethodPut, contentType: contentTypeSCIMJSON, mark: true, read: true, wantReqBody: true},
		{name: "POST with charset", level: auditlogv1.LevelRequest, method: http.MethodPost, contentType: contentTypeSCIMJSON + "; charset=utf-8", mark: true, read: true, wantReqBody: true},
		{name: "PATCH with JSON", level: auditlogv1.LevelRequest, method: http.MethodPatch, contentType: contentTypeJSON, mark: true, read: true, wantReqBody: true},
		{name: "PUT with JSON", level: auditlogv1.LevelRequest, method: http.MethodPut, contentType: contentTypeJSON, mark: true, read: true, wantReqBody: true},
		{name: "response body", level: auditlogv1.LevelRequestResponse, method: http.MethodPatch, contentType: contentTypeSCIMJSON, mark: true, read: true, wantReqBody: true, wantResBody: true},
		{name: "below request level", level: auditlogv1.LevelHeaders, method: http.MethodPatch, contentType: contentTypeSCIMJSON, mark: true, read: true},
		// SCIM entries log only what the handler read, not the copy the existing JSON rules read before routing.
		{name: "body not read", level: auditlogv1.LevelRequest, method: http.MethodPut, contentType: contentTypeJSON, mark: true},
		{name: "other content type", level: auditlogv1.LevelRequestResponse, method: http.MethodPatch, contentType: "text/plain", mark: true, read: true},
		{name: "not marked", level: auditlogv1.LevelRequestResponse, method: http.MethodPatch, contentType: contentTypeSCIMJSON, read: true},
		{name: "not marked PUT with JSON", level: auditlogv1.LevelRequest, method: http.MethodPut, contentType: contentTypeJSON, read: true, wantReqBody: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			writer, auditOutput := newTestAuditWriter(tt.level)

			handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				if tt.mark {
					CaptureSCIMRequest(req)
				}
				if tt.read {
					body, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					assert.Equal(t, reqBody, string(body))
				}

				rw.Header().Set("Content-Type", tt.contentType)
				rw.WriteHeader(http.StatusOK)
				rw.Write([]byte(resBody))
			})

			req := newTestRequest(tt.method, "/v1-scim/okta/Users/u-abc", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", tt.contentType)

			NewAuditLogMiddleware(writer)(handler).ServeHTTP(httptest.NewRecorder(), req)

			var entry map[string]any
			require.NoError(t, json.Unmarshal(auditOutput.Bytes(), &entry))

			if tt.wantReqBody {
				var want map[string]any
				require.NoError(t, json.Unmarshal([]byte(reqBody), &want))
				assert.Equal(t, want, entry["requestBody"])
			} else {
				assert.Nil(t, entry["requestBody"])
			}

			if tt.wantResBody {
				assert.Equal(t, map[string]any{"id": "u-abc", "active": false}, entry["responseBody"])
			} else {
				assert.Nil(t, entry["responseBody"])
			}
		})
	}
}

func TestCaptureSCIMRequestBelowRequestLevel(t *testing.T) {
	t.Parallel()

	writer, _ := newTestAuditWriter(auditlogv1.LevelHeaders)

	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		body := req.Body
		CaptureSCIMRequest(req)
		assert.True(t, body == req.Body, "the body must not be wrapped below the request level")
	})

	req := newTestRequest(http.MethodPatch, "/v1-scim/okta/Users/u-abc", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", contentTypeSCIMJSON)

	NewAuditLogMiddleware(writer)(handler).ServeHTTP(httptest.NewRecorder(), req)
}

func TestCaptureSCIMRequestOversizeBody(t *testing.T) {
	t.Parallel()

	const limit = 1024 * 1024 // the default public-api-body-limit
	rawBody := `{"data":"` + strings.Repeat("x", limit) + `"}`

	t.Run("handler reads the whole body", func(t *testing.T) {
		t.Parallel()

		writer, auditOutput := newTestAuditWriter(auditlogv1.LevelRequest)

		var read []byte
		handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			CaptureSCIMRequest(req)
			var err error
			read, err = io.ReadAll(req.Body)
			require.NoError(t, err)
		})

		req := newTestRequest(http.MethodPatch, "/v1-scim/okta/Users/u-abc", strings.NewReader(rawBody))
		req.Header.Set("Content-Type", contentTypeSCIMJSON)

		NewAuditLogMiddleware(writer)(handler).ServeHTTP(httptest.NewRecorder(), req)

		assert.True(t, rawBody == string(read), "handler read %d bytes, want the original %d", len(read), len(rawBody))

		var entry map[string]any
		require.NoError(t, json.Unmarshal(auditOutput.Bytes(), &entry))
		require.IsType(t, map[string]any{}, entry["requestBody"])
		assert.Contains(t, entry["requestBody"].(map[string]any)[auditLogErrorKey], "larger than")
	})

	t.Run("route body limit", func(t *testing.T) {
		t.Parallel()

		writer, auditOutput := newTestAuditWriter(auditlogv1.LevelRequest)

		// The SCIM authenticator runs inside the SCIM routes' http.MaxBytesHandler.
		var readErr error
		handler := http.MaxBytesHandler(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			CaptureSCIMRequest(req)
			_, readErr = io.ReadAll(req.Body)
			rw.WriteHeader(http.StatusBadRequest)
		}), limit)

		req := newTestRequest(http.MethodPatch, "/v1-scim/okta/Users/u-abc", strings.NewReader(rawBody))
		req.Header.Set("Content-Type", contentTypeSCIMJSON)

		NewAuditLogMiddleware(writer)(handler).ServeHTTP(httptest.NewRecorder(), req)

		var maxBytesErr *http.MaxBytesError
		assert.ErrorAs(t, readErr, &maxBytesErr)

		var entry map[string]any
		require.NoError(t, json.Unmarshal(auditOutput.Bytes(), &entry))
		require.IsType(t, map[string]any{}, entry["requestBody"])
		assert.Contains(t, entry["requestBody"].(map[string]any)[auditLogErrorKey], "request body too large")
	})
}

func TestCaptureSCIMRequestTwice(t *testing.T) {
	t.Parallel()

	const reqBody = `{"Operations":[{"op":"replace","path":"active","value":false}]}`

	writer, auditOutput := newTestAuditWriter(auditlogv1.LevelRequest)

	handler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		CaptureSCIMRequest(req)
		CaptureSCIMRequest(req)
		_, err := io.ReadAll(req.Body)
		require.NoError(t, err)
	})

	req := newTestRequest(http.MethodPatch, "/v1-scim/okta/Users/u-abc", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", contentTypeSCIMJSON)

	NewAuditLogMiddleware(writer)(handler).ServeHTTP(httptest.NewRecorder(), req)

	var entry, want map[string]any
	require.NoError(t, json.Unmarshal(auditOutput.Bytes(), &entry))
	require.NoError(t, json.Unmarshal([]byte(reqBody), &want))
	assert.Equal(t, want, entry["requestBody"])
}

func TestSCIMBodyReaderLimit(t *testing.T) {
	t.Parallel()

	const limit = 10

	tests := []struct {
		name          string
		bodySize      int
		wantKept      int
		wantOverLimit bool
	}{
		{name: "under the limit", bodySize: limit - 1, wantKept: limit - 1},
		{name: "exactly the limit", bodySize: limit, wantKept: limit},
		{name: "over the limit", bodySize: 100, wantKept: limit, wantOverLimit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rawBody := strings.Repeat("x", tt.bodySize)
			rec := &scimRecord{limit: limit}
			reader := &scimBodyReader{ReadCloser: io.NopCloser(iotest.OneByteReader(strings.NewReader(rawBody))), rec: rec}

			read, err := io.ReadAll(reader)
			require.NoError(t, err)

			assert.Equal(t, rawBody, string(read))
			assert.Equal(t, tt.wantKept, rec.body.Len())
			assert.Equal(t, tt.wantOverLimit, rec.overLimit)
		})
	}
}
