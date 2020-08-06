package channelserver

import (
	"context"
	"net/http/httputil"
	rurl "net/url"
)

func NewProxy(ctx context.Context) *httputil.ReverseProxy {
	const rawURL = "http://localhost:8115"
	cmdArgs := []string{"--config-key=k3s",
		"--config-key=rke2",
		"--path-prefix=v1-k3s-release",
		"--path-prefix=v1-rke2-release",
		"--listen-address=0.0.0.0:8115"}
	go Start(ctx, cmdArgs)
	url, _ := rurl.Parse(rawURL)
	return httputil.NewSingleHostReverseProxy(url)
}
