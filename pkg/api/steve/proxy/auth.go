package proxy

import (
	"net/http"
	"strings"

	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
)

const (
	tokenIndex = "clusterToken"
	Prefix     = "stv-cluster-"
)

type clusterProxyAuthorizer struct {
	secretCache corecontrollers.SecretCache
}

func NewAuthorizer(wrangler *wrangler.Context) remotedialer.Authorizer {
	secretCache := wrangler.Core.Secret().Cache()
	a := &clusterProxyAuthorizer{
		secretCache: secretCache,
	}
	secretCache.AddIndexer(tokenIndex, func(obj *corev1.Secret) ([]string, error) {
		return crt.SecretTokenIndexValues(obj), nil
	})

	return a.Authorize
}

func (a *clusterProxyAuthorizer) Authorize(req *http.Request) (string, bool, error) {
	auth := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(auth, Prefix) {
		return "", false, nil
	}
	secrets, err := a.secretCache.GetByIndex(tokenIndex, strings.TrimPrefix(auth, Prefix))
	if apierror.IsNotFound(err) || len(secrets) == 0 {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	return Prefix + secrets[0].Namespace, true, nil
}
