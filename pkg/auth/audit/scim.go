package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/rancher/rancher/pkg/settings"
)

const contentTypeSCIMJSON = "application/scim+json"

type scimRecordKey struct{}

// scimRecord holds the per-request state for SCIM requests, marked by [CaptureSCIMRequest].
// The audit middleware stores a pointer to it in the request context.
type scimRecord struct {
	keepBody bool // Whether the audit level requires request bodies.

	marked    bool
	body      bytes.Buffer
	limit     int
	overLimit bool
	readErr   error
}

// CaptureSCIMRequest marks r as a SCIM request for the audit log. The SCIM server calls it after authenticating the request.
// If the audit level requires request bodies, it wraps r.Body so the audit log keeps a copy of up to public-api-body-limit bytes
// as the handler reads them. The handler reads the original body unchanged.
// It does nothing if audit logging is disabled.
func CaptureSCIMRequest(r *http.Request) {
	rec, ok := r.Context().Value(scimRecordKey{}).(*scimRecord)
	if !ok || rec.marked {
		return
	}

	rec.marked = true
	if !rec.keepBody || r.Body == nil || r.Body == http.NoBody {
		return
	}

	rec.limit = int(apiBodyLimit())
	r.Body = &scimBodyReader{ReadCloser: r.Body, rec: rec}
}

// scimBodyReader copies what the handler reads into the record.
type scimBodyReader struct {
	io.ReadCloser
	rec *scimRecord
}

func (b *scimBodyReader) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.rec.write(p[:n])
	if err != nil && !errors.Is(err, io.EOF) && b.rec.readErr == nil {
		b.rec.readErr = err
	}

	return n, err
}

func (r *scimRecord) write(p []byte) {
	if room := r.limit - r.body.Len(); len(p) > room {
		r.overLimit = true
		p = p[:max(room, 0)]
	}
	r.body.Write(p)
}

// requestBody returns the request body for the audit log entry, or nil if the handler read none.
func (r *scimRecord) requestBody() map[string]any {
	switch {
	case r.readErr != nil:
		return map[string]any{auditLogErrorKey: fmt.Sprintf("failed to read request body: %s", r.readErr)}
	case r.overLimit:
		return map[string]any{auditLogErrorKey: fmt.Sprintf("request body is larger than %d bytes", r.limit)}
	case r.body.Len() == 0:
		return nil
	}

	var body map[string]any
	if err := json.Unmarshal(r.body.Bytes(), &body); err != nil {
		return map[string]any{auditLogErrorKey: fmt.Sprintf("failed to unmarshal request body: %s", err)}
	}

	return body
}

// isLoggableJSON reports whether a body with contentType is parsed into the audit log.
// SCIM entries also accept the SCIM media type and media type parameters, such as charset.
func isLoggableJSON(contentType string, scim bool) bool {
	if contentType == contentTypeJSON {
		return true
	}
	if !scim {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mediaType == contentTypeJSON || mediaType == contentTypeSCIMJSON)
}

func apiBodyLimit() int64 {
	limit, err := settings.APIBodyLimit.GetQuantityAsInt64(1024 * 1024)
	if err != nil {
		return 1024 * 1024
	}

	return limit
}
