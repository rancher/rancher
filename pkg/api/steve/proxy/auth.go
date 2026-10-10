package proxy

import (
	"net/http"
	"strings"

	crt "github.com/rancher/rancher/pkg/controllers/dashboard/clusterregistrationtoken"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/tunnelserver"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
)

const (
	tokenIndex = "clusterToken"
	Prefix     = "stv-cluster-"
)

type clusterProxyAuthorizer struct {
	secretCache    corecontrollers.SecretCache
	namespaceCache corecontrollers.NamespaceCache
	clusterCache   mgmtcontrollers.ClusterCache
}

func NewAuthorizer(wrangler *wrangler.Context) remotedialer.Authorizer {
	secretCache := wrangler.Core.Secret().Cache()
	a := &clusterProxyAuthorizer{
		secretCache:    secretCache,
		namespaceCache: wrangler.Core.Namespace().Cache(),
		clusterCache:   wrangler.Mgmt.Cluster().Cache(),
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

	for _, secret := range secrets {
		cluster, stale, err := crt.TokenSecretCluster(secret, a.namespaceCache.Get, a.clusterCache.Get)
		if err != nil {
			return "", false, err
		}
		if stale {
			logrus.Debugf("[steve-proxy] rejecting registration token that belongs to a previous cluster named %s", secret.Namespace)
			continue
		}
		if cluster == nil {
			// The session must be tracked by the cluster's UID, so that it is ended once that cluster is
			// gone, instead of serving the name for a cluster that replaces it.
			logrus.Debugf("[steve-proxy] rejecting registration token of cluster %s, which can't be found", secret.Namespace)
			continue
		}
		// Track the session for the cluster the token was checked against.
		tunnelserver.SetSessionCluster(req, cluster.Name, cluster.UID)
		return Prefix + cluster.Name, true, nil
	}
	return "", false, nil
}
