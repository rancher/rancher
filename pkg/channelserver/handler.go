package channelserver

import (
	"context"
	"net/http/httputil"
	rurl "net/url"
)

var rawURL = "http://localhost:8115"

func NewProxy(ctx context.Context) *httputil.ReverseProxy {
	go Start(ctx)
	url, _ := rurl.Parse(rawURL)
	return httputil.NewSingleHostReverseProxy(url)
}
