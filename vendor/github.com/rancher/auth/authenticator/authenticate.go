package authenticator

import (
	"context"
	"net/http"

	"k8s.io/client-go/tools/cache"

	"github.com/rancher/types/config"
)

type Authenticator interface {
	Authenticate(req *http.Request) (authed bool, user string, groups []string, err error)
}

func NewAuthenticator(ctx context.Context, mgmtCtx *config.ManagementContext) Authenticator {
	tokenInformer := mgmtCtx.Management.Tokens("").Controller().Informer()
	tokenInformer.AddIndexers(map[string]cache.IndexFunc{tokenKeyIndex: tokenKeyIndexer})

	return &tokenAuthenticator{
		ctx:          ctx,
		tokenIndexer: tokenInformer.GetIndexer(),
		tokenClient:  mgmtCtx.Management.Tokens(""),
	}
}
