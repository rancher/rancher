package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/stretchr/testify/assert"
)

type testLogData struct {
	verbosity  auditlogv1.LogVerbosity
	reqHeaders http.Header
	rawReqBody []byte
	resHeaders http.Header
	rawResBody []byte
}

// prepareLogEntry replicates necessary construction steps.
// With the refactor, newLog() does this automatically, but tests that create
// logEntry directly need to prepare them manually.
func prepareLogEntry(log *logEntry, data *testLogData) {
	if data.verbosity.Request.Headers {
		log.RequestHeader = data.reqHeaders
	}

	// Attempt req body prep
	if data.verbosity.Request.Body && data.reqHeaders.Get("Content-Type") == contentTypeJSON && len(data.rawReqBody) > 0 {
		if err := json.Unmarshal(data.rawReqBody, &log.RequestBody); err != nil {
			log.RequestBody = map[string]any{
				auditLogErrorKey: fmt.Sprintf("failed to unmarshal request body: %s", err.Error()),
			}
		}
	}

	if data.verbosity.Response.Headers {
		log.ResponseHeader = data.resHeaders
	}

	// Attempt res body prep
	if data.verbosity.Response.Body {
		log.prepareResponseBody(data.resHeaders, data.rawResBody)
	}
}

func TestMergePolicyVerbosities(t *testing.T) {
	type testCase struct {
		Name     string
		Lhs      auditlogv1.LogVerbosity
		Rhs      auditlogv1.LogVerbosity
		Expected auditlogv1.LogVerbosity
	}

	testCases := []testCase{
		{
			Name: "Zeroed Verbosities Returns Zeroed Verbosity",
		},
		{
			Name:     "Single Policy Returns Policy Verbosity",
			Rhs:      verbosityForLevel(auditlogv1.LevelHeaders),
			Expected: verbosityForLevel(auditlogv1.LevelHeaders),
		},
		{
			Name: "Multiple Policies Merge Verbosities",
			Lhs: auditlogv1.LogVerbosity{
				Request: auditlogv1.Verbosity{
					Headers: true,
				},
				Response: auditlogv1.Verbosity{
					Headers: true,
				},
			},
			Rhs: auditlogv1.LogVerbosity{
				Response: auditlogv1.Verbosity{
					Body: true,
				},
			},
			Expected: auditlogv1.LogVerbosity{
				Request: auditlogv1.Verbosity{
					Headers: true,
				},
				Response: auditlogv1.Verbosity{
					Headers: true,
					Body:    true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := mergeLogVerbosities(tc.Lhs, tc.Rhs)
			assert.Equal(t, tc.Expected, actual)
		})
	}
}

func TestHasKey(t *testing.T) {
	type testCase struct {
		Name     string
		Value    any
		Func     func(string, any) bool
		Expected bool
	}

	cases := []testCase{
		{
			Name:  "Empty Map Returns False",
			Value: map[string]any{},
			Func: func(k string, _ any) bool {
				return k == "foo"
			},
		},
		{
			Name:  "Empty Slice Returns False",
			Value: []any{},
			Func: func(k string, _ any) bool {
				return k == "foo"
			},
		},
		{
			Name:  "Map With Key Returns True",
			Value: map[string]any{"foo": nil},
			Func: func(k string, _ any) bool {
				return k == "foo"
			},
			Expected: true,
		},
		{
			Name:  "Map Without Key Returns False",
			Value: map[string]any{"foo": nil},
			Func: func(k string, _ any) bool {
				return k == "bar"
			},
			Expected: false,
		},
		{
			Name: "Nested Map With Key Returns True",
			Value: map[string]any{
				"foo": map[string]any{
					"bar": nil,
				},
			},
			Func: func(k string, _ any) bool {
				return k == "bar"
			},
			Expected: true,
		},
		{
			Name: "Map Slice With Key Returns True",
			Value: []any{
				map[string]any{"foo": nil},
				map[string]any{"bar": nil},
			},
			Func: func(k string, _ any) bool {
				return k == "foo"
			},
			Expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := pairMatches(tc.Value, tc.Func)
			assert.Equal(t, tc.Expected, actual)
		})
	}
}
