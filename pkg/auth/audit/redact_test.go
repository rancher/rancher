package audit

import (
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

func sampleLog() log {
	return log{
		RequestHeader: map[string][]string{
			"password": {"password1234"},
			"foo":      {"bar"},
		},
		ResponseHeader: map[string][]string{
			"password": {"password1234"},
			"baz":      {"qux"},
		},
		rawRequestBody:  []byte(`{"toplevel":{"inner":{"bottom":"value"},"sibling":"value"}}`),
		rawResponseBody: []byte(`{"words":[{"foo":"bar"},{"baz":"qux"}]}`),
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
		Input    log
		Expected log
	}

	cases := []testCase{
		{
			Name:     "Redact Headers",
			Redactor: headerRedactor,
			Input:    sampleLog(),
			Expected: log{
				RequestHeader: map[string][]string{
					"password": {redacted},
					"foo":      {"bar"},
				},
				ResponseHeader: map[string][]string{
					"password": {redacted},
					"baz":      {"qux"},
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
			Expected: log{
				RequestHeader: map[string][]string{
					"password": {"password1234"},
					"foo":      {"bar"},
				},
				ResponseHeader: map[string][]string{
					"password": {"password1234"},
					"baz":      {"qux"},
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
			Expected: log{
				RequestHeader: map[string][]string{
					"password": {"password1234"},
					"foo":      {"bar"},
				},
				ResponseHeader: map[string][]string{
					"password": {"password1234"},
					"baz":      {"qux"},
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
			c.Input.prepare()
			err := c.Redactor.Redact(&c.Input)

			actual := c.Input
			assert.NoError(t, err)
			assert.Equal(t, c.Expected, actual)
		})
	}
}
