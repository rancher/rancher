package tokens

import (
	"context"
	"time"

	"github.com/rancher/norman/clientbase"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

const intervalSeconds int64 = 3600

func StartPurgeDaemon(ctx context.Context, mgmt *config.ManagementContext) {
	p := &purger{
		tokenLister:      mgmt.Management.Tokens("").Controller().Lister(),
		tokens:           mgmt.Management.Tokens(""),
		samlTokensLister: mgmt.Management.SamlTokens("").Controller().Lister(),
		samlTokens:       mgmt.Management.SamlTokens(""),
	}
	go wait.JitterUntil(p.purge, time.Duration(intervalSeconds)*time.Second, .1, true, ctx.Done())
}

type purger struct {
	tokenLister      v3.TokenLister
	tokens           v3.TokenInterface
	samlTokens       v3.SamlTokenInterface
	samlTokensLister v3.SamlTokenLister
}

func (p *purger) purge() {
	allTokens, err := p.tokenLister.List("", labels.Everything())
	if err != nil {
		logrus.Errorf("Error listing tokens during purge: %v", err)
	}

	var count int
	for _, token := range allTokens {
		if IsExpired(*token) {
			err = p.tokens.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{})
			if err != nil && !clientbase.IsNotFound(err) {
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
	samlTokens, err := p.samlTokensLister.List(namespace.GlobalNamespace, labels.Everything())
	if err != nil {
		return
	}

	count = 0
	for _, token := range samlTokens {
		// avoid delete immediately after creation, login request might be pending
		if token.CreationTimestamp.Add(15 * time.Minute).Before(time.Now()) {
			err = p.samlTokens.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{})
			if err != nil && !clientbase.IsNotFound(err) {
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
