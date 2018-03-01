package clusteregistrationtokens

import (
	"net/http"

	"github.com/rancher/norman/types"
)

type ShellLinkHandler struct {
	Proxy http.Handler
}

func LinkHandler(apiContext *types.APIContext, next types.RequestHandler) error {
	return nil
}
