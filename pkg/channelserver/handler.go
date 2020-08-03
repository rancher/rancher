package channelserver

import (
	"context"
	"net/http/httputil"
	rurl "net/url"
)

var k3sURL = "http://localhost:8115"
var rke2URL = "http://localhost:8116"

func NewProxy(ctx context.Context) *httputil.ReverseProxy {
	go Start(ctx, "k3s", "8115", "v1-k3s-release")
	url, _ := rurl.Parse(k3sURL)
	return httputil.NewSingleHostReverseProxy(url)
}
func Rke2Proxy(ctx context.Context) *httputil.ReverseProxy {
	go Start(ctx, "rke2", "8116", "v1-rke2-release")
	url, _ := rurl.Parse(rke2URL)
	return httputil.NewSingleHostReverseProxy(url)
}
