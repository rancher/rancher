package audit

import (
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/stretchr/testify/assert"
)

func sampleLog() logEntry {
	return logEntry{
		RequestHeader: map[string][]string{
			"password":     {"password1234"},
			"foo":          {"bar"},
			"Content-Type": []string{contentTypeJSON},
		},
		ResponseHeader: map[string][]string{
			"password":     {"password1234"},
			"baz":          {"qux"},
			"Content-Type": []string{contentTypeJSON},
		},
		RequestBody: map[string]any{
			"toplevel": map[string]any{
				"inner":   map[string]any{"bottom": "value"},
				"sibling": "value",
			},
		},
		ResponseBody: map[string]any{
			"words": []any{
				map[string]any{"foo": "bar"},
				map[string]any{"baz": "qux"},
			},
		},
	}
}

func TestPolicyRedactor(t *testing.T) {
	headerRedactor, err := NewRedactor(auditlogv1.Redaction{
		Headers: []string{"password"},
	})
	assert.NoError(t, err)

	pathRedactor, err := NewRedactor(auditlogv1.Redaction{
		Paths: []string{"$.toplevel.inner", "$.words[*].baz"},
	})
	assert.NoError(t, err)

	keyRedactor, err := NewRedactor(auditlogv1.Redaction{
		Paths: []string{"$..[foo,bar,baz]"},
	})
	assert.NoError(t, err)

	type testCase struct {
		Name     string
		Redactor *policyRedactor
		Input    logEntry
		Expected logEntry
	}

	cases := []testCase{
		{
			Name:     "Redact Headers",
			Redactor: headerRedactor,
			Input:    sampleLog(),
			Expected: logEntry{
				RequestHeader: map[string][]string{
					"password":     {redacted},
					"foo":          {"bar"},
					"Content-Type": []string{contentTypeJSON},
				},
				ResponseHeader: map[string][]string{
					"password":     {redacted},
					"baz":          {"qux"},
					"Content-Type": []string{contentTypeJSON},
				},
				RequestBody: map[string]any{
					"toplevel": map[string]any{
						"inner": map[string]any{"bottom": "value"}, "sibling": "value",
					},
				},
				ResponseBody: map[string]any{
					"words": []any{
						map[string]any{"foo": "bar"},
						map[string]any{"baz": "qux"},
					},
				},
			},
		},
		{
			Name:     "Redact Both With Paths",
			Redactor: pathRedactor,
			Input:    sampleLog(),
			Expected: logEntry{
				RequestHeader: map[string][]string{
					"password":     {"password1234"},
					"foo":          {"bar"},
					"Content-Type": []string{contentTypeJSON},
				},
				ResponseHeader: map[string][]string{
					"password":     {"password1234"},
					"baz":          {"qux"},
					"Content-Type": []string{contentTypeJSON},
				},
				RequestBody: map[string]any{
					"toplevel": map[string]any{
						"inner":   redacted,
						"sibling": "value",
					},
				},
				ResponseBody: map[string]any{
					"words": []any{
						map[string]any{"foo": "bar"},
						map[string]any{"baz": redacted},
					},
				},
			},
		},
		{
			Name:     "Redact Keys Regex",
			Redactor: keyRedactor,
			Input:    sampleLog(),
			Expected: logEntry{
				RequestHeader: map[string][]string{
					"password":     {"password1234"},
					"foo":          {"bar"},
					"Content-Type": []string{contentTypeJSON},
				},
				ResponseHeader: map[string][]string{
					"password":     {"password1234"},
					"baz":          {"qux"},
					"Content-Type": []string{contentTypeJSON},
				},
				RequestBody: map[string]any{
					"toplevel": map[string]any{
						"inner":   map[string]any{"bottom": "value"},
						"sibling": "value",
					},
				},
				ResponseBody: map[string]any{
					"words": []any{
						map[string]any{"foo": redacted},
						map[string]any{"baz": redacted},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			// No need to call prepare() - test inputs already have bodies as maps
			err := c.Redactor.Redact(&c.Input)

			actual := c.Input
			assert.NoError(t, err)
			assert.Equal(t, c.Expected, actual)
		})
	}
}

const (
	redactableV3URL       = "/v3/import/redactME.yaml"
	expectedV3RedactedURL = "/v3/import/[redacted]"
)

func Test_redactImportUrlPath(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{
			"/v3/import",
			expectedV3RedactedURL,
		},
		{
			"/v3/import/",
			expectedV3RedactedURL,
		},
		{
			redactableV3URL,
			expectedV3RedactedURL,
		},
		{
			"/foo/bar" + redactableV3URL,
			"/foo/bar" + redactableV3URL,
		},
		{
			"/v3/import/yellow.yaml",
			expectedV3RedactedURL,
		},
		{
			"/v4/import/redactME.yaml",
			"/v4/import/redactME.yaml",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, redactImportUrlPath(tt.input))
		})
	}

	noRedaction := redactImportUrlPath("/v4/import/redactME.yaml")
	assert.Equal(t, noRedaction, "/v4/import/redactME.yaml")
	assert.NotEqual(t, noRedaction, expectedV3RedactedURL)
}

const testingServerURL = "https://127.0.0.1.sslip.io:8443"

func Test_redactImportUrlString(t *testing.T) {
	asserts := assert.New(t)

	cases := []struct {
		input     string
		serverUrl string
		expected  string
	}{
		{
			testingServerURL + "/v3/import",
			testingServerURL,
			testingServerURL + expectedV3RedactedURL,
		},
		{
			testingServerURL + "/v3/import/",
			testingServerURL,
			testingServerURL + expectedV3RedactedURL,
		},
		{
			testingServerURL + redactableV3URL,
			testingServerURL,
			testingServerURL + expectedV3RedactedURL,
		},
		{
			testingServerURL + "/foo/bar" + redactableV3URL,
			testingServerURL,
			testingServerURL + "/foo/bar" + redactableV3URL,
		},
		{
			testingServerURL + "/v3/import/yellow.yaml",
			testingServerURL,
			testingServerURL + expectedV3RedactedURL,
		},
		{
			testingServerURL + "/v4/import/will-not-redactME.yaml",
			testingServerURL,
			testingServerURL + "/v4/import/will-not-redactME.yaml",
		},
		{
			"https://some-other-domain.localhost/v4/import/will-not-redactME.yaml",
			testingServerURL,
			"https://some-other-domain.localhost/v4/import/will-not-redactME.yaml",
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			originalServerURL := settings.ServerURL.Get()

			_ = settings.ServerURL.Set(tt.serverUrl)
			asserts.Equal(tt.expected, redactImportUrlString(tt.input))

			t.Cleanup(func() {
				_ = settings.ServerURL.Set(originalServerURL)
			})
		})
	}

}

func Test_redactImportUrl(t *testing.T) {
	asserts := assert.New(t)
	originalServerURL := settings.ServerURL.Get()

	testLog := sampleLog()
	asserts.Equal("", testLog.RequestURI)

	referrerHeader := testLog.RequestHeader.Get(refererHeader)
	asserts.Equal("", referrerHeader)

	_ = settings.ServerURL.Set(testingServerURL)

	testLog.RequestURI = redactableV3URL
	asserts.Equal(redactableV3URL, testLog.RequestURI)

	testLog.RequestHeader.Set(refererHeader, testingServerURL+redactableV3URL)
	referrerHeader = testLog.RequestHeader.Get(refererHeader)
	asserts.Equal(testingServerURL+redactableV3URL, referrerHeader)

	err := redactImportUrl(&testLog)
	asserts.NoError(err)
	asserts.NotEqual(redactableV3URL, testLog.RequestURI)
	asserts.Equal(expectedV3RedactedURL, testLog.RequestURI)

	referrerHeader = testLog.RequestHeader.Get(refererHeader)
	asserts.NotEqual(redactableV3URL, referrerHeader)
	asserts.Equal(testingServerURL+expectedV3RedactedURL, referrerHeader)

	t.Cleanup(func() {
		_ = settings.ServerURL.Set(originalServerURL)
	})
}
