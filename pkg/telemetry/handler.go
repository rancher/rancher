package telemetry

import (
	"net/http/httputil"
	rurl "net/url"
)

var rawURL = "http://localhost:8114"

func NewProxy() *httputil.ReverseProxy {
	url, _ := rurl.Parse(rawURL)
	return httputil.NewSingleHostReverseProxy(url)
}
