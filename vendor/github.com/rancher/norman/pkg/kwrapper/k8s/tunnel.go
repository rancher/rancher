package k8s

import (
	"net/http"

	"github.com/rancher/norman/pkg/remotedialer"
)

func newTunnel(authorizer remotedialer.Authorizer) http.Handler {
	if authorizer == nil {
		return nil
	}
	server := remotedialer.New(authorizer, remotedialer.DefaultErrorWriter)
	setupK3s(server)
	return server
}
