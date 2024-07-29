package capturewindowclient

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func Test_RoundTrip(t *testing.T) {
	type testcase struct {
		name            string
		expectedErr     string
		headervalue     string
		backOffDuration float64
		retryHeader     bool
	}

	testCase1 := testcase{
		name:            "sets backoff duration when response status code is 429 with header",
		headervalue:     "0;w=100",
		backOffDuration: 100,
		expectedErr:     "",
		retryHeader:     false,
	}
	testCase2 := testcase{
		name:            "doesn't set backoff duration when response status code is 429 with header value empty",
		headervalue:     "",
		backOffDuration: 0.0,
		expectedErr:     "",
		retryHeader:     false,
	}
	testCase3 := testcase{
		name:            "return error when invalid header value present",
		headervalue:     "w=121",
		backOffDuration: 0.0,
		expectedErr:     "the format of ratelimited header is wrong",
		retryHeader:     false,
	}
	testCase4 := testcase{
		name:            "sets backoff duration when response status code is 429 with retry after header",
		headervalue:     "123",
		backOffDuration: 123,
		expectedErr:     "",
		retryHeader:     true,
	}

	testCases := []testcase{
		testCase1,
		testCase2,
		testCase3,
		testCase4,
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logrus.SetOutput(&buf)

			// Create a test server that returns a 429 response with the RateLimit-Remaining header set to 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.retryHeader {
					w.Header().Set(RetryAfterHeader, tc.headervalue)
				} else {
					w.Header().Set(RateLimitRemainingHeader, tc.headervalue)
				}
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()

			transport := NewTransport(server.Client().Transport)

			// Create a request to be sent using the Transport
			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			assert.Nil(err)

			// Send the request using the Transport
			resp, err := transport.RoundTrip(req)
			assert.Equal(http.StatusTooManyRequests, resp.StatusCode)
			assert.Equal(tc.backOffDuration, transport.BackOffDuration)
			if tc.expectedErr != "" {
				assert.Contains(buf.String(), tc.expectedErr)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func Test_RoundTripHeadRequest(t *testing.T) {
	type testcase struct {
		name            string
		expectedErr     string
		headervalue     string
		backOffDuration float64
	}

	testCase1 := testcase{
		name:            "sets backoff duration when response status code is 429 with head request header",
		headervalue:     "0;w=200",
		backOffDuration: 200,
		expectedErr:     "",
	}
	testCase2 := testcase{
		name:            "return error when invalid header value present",
		headervalue:     "w=121",
		backOffDuration: 0.0,
		expectedErr:     "the format of ratelimited header is wrong",
	}
	testCase3 := testcase{
		name:            "doesn't set backoff duration when response status code is 429 with header value empty",
		headervalue:     "",
		backOffDuration: 0.0,
		expectedErr:     "",
	}

	testCases := []testcase{
		testCase1,
		testCase2,
		testCase3,
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logrus.SetOutput(&buf)

			// Create a test server that returns a 429 response with the RateLimit-Remaining header set to 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusTooManyRequests)
				} else if r.Method == http.MethodHead {
					w.Header().Set(RateLimitRemainingHeader, tc.headervalue)
					w.WriteHeader(http.StatusTooManyRequests)
				}
			}))
			defer server.Close()

			transport := NewTransport(server.Client().Transport)

			// Create a request to be sent using the Transport
			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			assert.Nil(err)

			// Make the request using the Transport
			resp, err := transport.RoundTrip(req)
			assert.Equal(http.StatusTooManyRequests, resp.StatusCode)
			assert.Equal(tc.backOffDuration, transport.BackOffDuration)
			if tc.expectedErr != "" {
				assert.Contains(buf.String(), tc.expectedErr)
			} else {
				assert.Nil(err)
			}
		})
	}
}
