package proxy

import (
	"net/http"
	"strings"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	apierror "k8s.io/apimachinery/pkg/api/errors"
)

const (
	tokenIndex = "clusterToken"
	Prefix     = "stv-cluster-"
)

type clusterProxyAuthorizer struct {
	tokenCache  v3.ClusterRegistrationTokenCache
	secretCache corecontrollers.SecretCache
}

func NewAuthorizer(wrangler *wrangler.Context) remotedialer.Authorizer {
	secretCache := wrangler.Core.Secret().Cache()
	a := &clusterProxyAuthorizer{
		tokenCache:  wrangler.Mgmt.ClusterRegistrationToken().Cache(),
		secretCache: secretCache,
	}
	a.tokenCache.AddIndexer(tokenIndex, func(obj *apimgmtv3.ClusterRegistrationToken) ([]string, error) {
		current, previous, err := crt.GetTokensFromSecret(secretCache, obj)
		if err != nil {
			logrus.Warnf("failed to resolve CRT token for %s/%s: %v", obj.Namespace, obj.Name, err)
			return nil, nil
		}
		if current == "" {
			return nil, nil
		}
		tokens := []string{current}
		if previous != "" {
			tokens = append(tokens, previous)
		}
		return tokens, nil
	})

	return a.Authorize
}

func (a *clusterProxyAuthorizer) Authorize(req *http.Request) (string, bool, error) {
	auth := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(auth, Prefix) {
		return "", false, nil
	}
	crts, err := a.tokenCache.GetByIndex(tokenIndex, strings.TrimPrefix(auth, Prefix))
	if apierror.IsNotFound(err) || len(crts) == 0 {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	return Prefix + crts[0].Namespace, true, nil
}
