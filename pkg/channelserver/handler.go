package channelserver

import (
	"context"
	"net/http/httputil"
	rurl "net/url"
)

func NewProxy(ctx context.Context) *httputil.ReverseProxy {
	const rawURL = "http://localhost:8115"
	go Start(ctx)
	url, _ := rurl.Parse(rawURL)
	return httputil.NewSingleHostReverseProxy(url)
}
