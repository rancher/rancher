package tokens

import (
	"context"
	"time"

	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

const intervalSeconds int64 = 3600

func StartPurgeDaemon(ctx context.Context, mgmt *config.ManagementContext) {
	p := &purger{
		tokenCache:       mgmt.Wrangler.Mgmt.Token().Cache(),
		tokens:           mgmt.Wrangler.Mgmt.Token(),
		samlTokensCache:  mgmt.Wrangler.Mgmt.SamlToken().Cache(),
		samlTokens:       mgmt.Wrangler.Mgmt.SamlToken(),
	}
	go wait.JitterUntil(p.purge, time.Duration(intervalSeconds)*time.Second, .1, true, ctx.Done())
}

type purger struct {
	tokenCache      mgmtv3.TokenCache
	tokens          mgmtv3.TokenClient
	samlTokens      mgmtv3.SamlTokenClient
	samlTokensCache mgmtv3.SamlTokenCache
}

func (p *purger) purge() {
	allTokens, err := p.tokenCache.List(labels.Everything())
	if err != nil {
		logrus.Errorf("Error listing tokens during purge: %v", err)
	}

	var count int
	for _, token := range allTokens {
		if IsExpired(token) {
			err = p.tokens.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("Error: while deleting expired token %v: %v", err, token.ObjectMeta.Name)
				continue
			}
			count++
		}
	}
	if count > 0 {
		logrus.Infof("Purged %v expired tokens", count)
	}

	// saml tokens store encrypted token for login request from rancher cli
	samlTokens, err := p.samlTokensCache.List(namespace.GlobalNamespace, labels.Everything())
	if err != nil {
		return
	}

	count = 0
	for _, token := range samlTokens {
		// avoid delete immediately after creation, login request might be pending
		if token.CreationTimestamp.Add(15 * time.Minute).Before(time.Now()) {
			err = p.samlTokens.Delete(namespace.GlobalNamespace, token.ObjectMeta.Name, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				logrus.Errorf("Error: while deleting expired token %v: %v", err, token.Name)
				continue
			}
			count++
		}
	}
	if count > 0 {
		logrus.Infof("Purged %v saml tokens", count)
	}
}
