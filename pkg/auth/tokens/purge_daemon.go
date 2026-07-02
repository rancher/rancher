package tokens

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rancher/norman/clientbase"
	exttokenstore "github.com/rancher/rancher/pkg/ext/stores/tokens"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
)

// intervalSeconds defines the interval at which the purge daemon runs.
const intervalSeconds int64 = 3600 // 1h.

// extTokenPurger deletes expired ext.Tokens.
type extTokenPurger interface {
	DeleteExpired() (int, error)
}

// startOnce guards StartPurgeDaemon so the daemon runs exactly once per
// process. Rancher wires OnLeader under two distinct lock names (auth server
// and multi-cluster manager); both fire on the same pod in single-instance
// deployments and would otherwise spin up two purge goroutines.
var startOnce sync.Once

func StartPurgeDaemon(ctx context.Context, mgmt *config.ManagementContext) {
	startOnce.Do(func() {
		p := &purger{
			tokenLister:      mgmt.Management.Tokens("").Controller().Lister(),
			tokens:           mgmt.Management.Tokens(""),
			samlTokensLister: mgmt.Management.SamlTokens("").Controller().Lister(),
			samlTokens:       mgmt.Management.SamlTokens(""),
			extTokenPurger:   exttokenstore.NewSystemFromWrangler(mgmt.Wrangler),
		}
		go wait.JitterUntil(p.purge, time.Duration(intervalSeconds)*time.Second, .1, true, ctx.Done())
	})
}

type purger struct {
	tokenLister      v3.TokenLister
	tokens           v3.TokenInterface
	samlTokens       v3.SamlTokenInterface
	samlTokensLister v3.SamlTokenLister
	extTokenPurger   extTokenPurger
}

func (p *purger) purge() {
	count, err := p.extTokenPurger.DeleteExpired()
	if err != nil {
		logrus.Errorf("Error purging ext tokens: %v", err)
	}
	if count > 0 {
		logrus.Infof("Purged %v expired ext tokens", count)
	}

	count, err = p.deleteExpiredV3Tokens()
	if err != nil {
		logrus.Errorf("Error purging v3 tokens: %v", err)
	}
	if count > 0 {
		logrus.Infof("Purged %v expired tokens", count)
	}

	count, err = p.deleteExpiredSamlTokens()
	if err != nil {
		logrus.Errorf("Error purging saml tokens: %v", err)
	}
	if count > 0 {
		logrus.Infof("Purged %v saml tokens", count)
	}
}

// deleteExpiredV3Tokens removes every v3.Token whose TTL has passed and
// returns the number of tokens deleted. A failure to list tokens is returned
// as-is; the returned count is 0 in that case. Per-token delete failures do
// not stop iteration; each is wrapped with the token name and joined into the
// returned error, and the count reflects only tokens that were actually
// deleted (or already gone).
func (p *purger) deleteExpiredV3Tokens() (int, error) {
	allV3Tokens, err := p.tokenLister.List("", labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("listing v3 tokens: %w", err)
	}

	var (
		count int
		errs  []error
	)
	for _, token := range allV3Tokens {
		if !IsExpired(token) {
			continue
		}
		if err := p.tokens.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{}); err != nil && !clientbase.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting %s: %w", token.ObjectMeta.Name, err))
			continue
		}
		count++
	}

	return count, errors.Join(errs...)
}

// deleteExpiredSamlTokens removes every SamlToken older than 15 minutes and
// returns the number of tokens deleted. The 15-minute grace period avoids
// racing with a pending login request from the Rancher CLI. Error semantics
// match [purger.deleteExpiredV3Tokens].
func (p *purger) deleteExpiredSamlTokens() (int, error) {
	samlTokens, err := p.samlTokensLister.List(namespace.GlobalNamespace, labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("listing saml tokens: %w", err)
	}

	var (
		count int
		errs  []error
	)
	for _, token := range samlTokens {
		if !token.CreationTimestamp.Add(15 * time.Minute).Before(time.Now()) {
			continue
		}
		if err := p.samlTokens.Delete(token.ObjectMeta.Name, &metav1.DeleteOptions{}); err != nil && !clientbase.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting %s: %w", token.ObjectMeta.Name, err))
			continue
		}
		count++
	}

	return count, errors.Join(errs...)
}
