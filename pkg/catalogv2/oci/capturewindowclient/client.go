package capturewindowclient

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	RateLimitRemainingHeader = "RateLimit-Remaining"
	RetryAfterHeader         = "Retry-After"
	TimeWindowPrefix         = "w="
)

// Transport is an HTTP transport with capturing duration to wait when 429 is returned by the server.
type Transport struct {
	// Base is the underlying HTTP transport to use.
	// If nil, http.DefaultTransport is used for round trips.
	Base http.RoundTripper

	// BackOffDuration is used as a duration to wait for by the
	// OCI handler until the next retry to make a call to the OCI registry.
	BackOffDuration float64
}

// NewTransport creates an HTTP Transport with the ability to store duration to wait when 429 is returned by the server.
func NewTransport(base http.RoundTripper) *Transport {
	return &Transport{
		Base: base,
	}
}

// RoundTrip executes a single HTTP transaction, returning a Response for the
// provided Request and it captures the duration to wait until the server unblocks
// the user to make more requests. It checks the headers to capture the duration.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, respErr := t.Base.RoundTrip(req)
	// If we hit 429 status code, try to capture the duration using available headers.
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		// Case 1: Check if RateLimit-Remaining header is present and get value from it.
		if v := resp.Header.Get(RateLimitRemainingHeader); v != "" {
			timeWindow, err := decodeTimeFromHeaderValue(v)
			if err != nil {
				logrus.Error("error:", err)
				return resp, respErr
			}
			t.BackOffDuration = timeWindow
			// Case 2: Check if Retry-After header is present and get value from it.
		} else if v := resp.Header.Get(RetryAfterHeader); v != "" {
			retryAfter, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				logrus.Error("error:", err)
				return resp, respErr
			}
			t.BackOffDuration = float64(retryAfter)
		} else {
			// Case 3: Dockerhub: It sends duration information when a HEAD call is made to the manifest.
			URL := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.Host, req.URL.Path)
			headRequest, err := http.NewRequest(http.MethodHead, URL, nil)
			if err != nil {
				logrus.Errorf("oci: failed to create a head request to %s:%v", URL, err)
				return resp, respErr
			}
			headRequest.Header.Add("Authorization", req.Header.Get("Authorization"))
			headResponse, err := t.Base.RoundTrip(headRequest)
			if err != nil {
				logrus.Errorf("oci: failed to make a head request to %s:%v", URL, err)
				return resp, respErr
			}

			// Decode the time window
			headerValue := headResponse.Header.Get(RateLimitRemainingHeader)
			if headerValue != "" {
				timeWindow, err := decodeTimeFromHeaderValue(headerValue)
				if err != nil {
					logrus.Error("error:", err)
					return resp, respErr
				}
				t.BackOffDuration = timeWindow
			}
		}
	}

	return resp, respErr
}

// decodeTimeFromHeaderValue decodes the time window from the header value.
func decodeTimeFromHeaderValue(value string) (float64, error) {
	newValue := strings.Replace(value, TimeWindowPrefix, "", 2)
	splitValue := strings.Split(newValue, ";")
	if len(splitValue) > 1 {
		remainingCount, err := strconv.ParseFloat(splitValue[1], 64)
		return remainingCount, err
	}
	return 0.0, fmt.Errorf("the format of ratelimited header is wrong %s", value)
}
