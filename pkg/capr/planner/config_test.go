package planner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractConfigJSON(t *testing.T) {
	tests := []struct {
		name           string
		extracted      string
		expected       map[string]interface{}
		errContains    string
		assertResultFn func(t *testing.T, result map[string]interface{})
	}{
		{
			name: "extracts machine config json from tar.gz archive",
			extracted: buildExtractConfigArchive(t, map[string][]byte{
				"state/machines/node-1/config.json": []byte(`{"DriverName":"amazonec2","Driver":{"IPAddress":"1.2.3.4","PrivateIPAddress":"10.0.0.10"}}`),
				"state/machines/node-1/other.txt":   []byte("ignored"),
			}),
			assertResultFn: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "amazonec2", result["DriverName"])
				driver := result["Driver"].(map[string]interface{})
				assert.Equal(t, "1.2.3.4", driver["IPAddress"])
				assert.Equal(t, "10.0.0.10", driver["PrivateIPAddress"])
			},
		},
		{
			name: "returns empty result when archive has no matching config.json",
			extracted: buildExtractConfigArchive(t, map[string][]byte{
				"state/machines/node-1/not-config.json": []byte(`{"ignored":true}`),
				"state/cluster/config.json":             []byte(`{"ignored":true}`),
			}),
			expected: map[string]interface{}{},
		},
		{
			name:        "fails on invalid base64",
			extracted:   "not-valid-base64",
			errContains: "base64.DecodeString",
		},
		{
			name:        "fails on non-gzip payload",
			extracted:   base64.StdEncoding.EncodeToString([]byte("plain text")),
			errContains: "gzip",
		},
		{
			name:        "fails on invalid tar payload",
			extracted:   buildExtractConfigGzipPayload(t, []byte("invalid tar payload")),
			errContains: "tarRead.Next",
		},
		{
			name: "fails when matching config json is invalid",
			extracted: buildExtractConfigArchive(t, map[string][]byte{
				"state/machines/node-1/config.json": []byte(`{"Driver":`),
			}),
			errContains: "failed to read config.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractConfigJSON(tt.extracted)
			if tt.errContains != "" {
				assert.Error(t, err)
				if err != nil {
					assert.True(t, strings.Contains(err.Error(), tt.errContains), fmt.Sprintf("expected error to contain %q, got %q", tt.errContains, err.Error()))
				}
				return
			}

			assert.NoError(t, err)
			if tt.assertResultFn != nil {
				tt.assertResultFn(t, result)
				return
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func buildExtractConfigArchive(t *testing.T, files map[string][]byte) string {
	t.Helper()

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gz)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0600,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func buildExtractConfigGzipPayload(t *testing.T, payload []byte) string {
	t.Helper()

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if _, err := gz.Write(payload); err != nil {
		t.Fatalf("failed to write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("failed to close gzip payload writer: %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
