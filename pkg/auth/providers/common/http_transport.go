package common

import (
	"net"
	"net/http"
	"time"
)

// NewHTTPClientWithTimeouts creates and returns a new HTTP Client with a
// transport configured with timeouts to prevent requests hanging.
func NewHTTPClientWithTimeouts() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}
