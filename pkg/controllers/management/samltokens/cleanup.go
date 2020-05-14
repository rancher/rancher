package samltokens

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	cleanupDuration = 30 * time.Minute
)

type cleanupHandler struct {
	tokens       v3.SamlTokenInterface
	tokensLister v3.SamlTokenLister
}

func Register(ctx context.Context, mgmt *config.ManagementContext) {
	ch := &cleanupHandler{
		tokens:       mgmt.Management.SamlTokens(""),
		tokensLister: mgmt.Management.SamlTokens("").Controller().Lister(),
	}

	go ch.cleanupTokens(ctx)

}

func (ch *cleanupHandler) cleanupTokens(ctx context.Context) {
	ticker := time.NewTicker(cleanupDuration)

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			logrus.Infof("received sync!!!!!!!!!!!!!")

			tokens, err := ch.tokensLister.List(namespace.GlobalNamespace, labels.Everything())
			if err != nil {
				return
			}

			for _, token := range tokens {
				if token.CreationTimestamp.Add(15 * time.Minute).Before(time.Now()) {
					logrus.Debugf("kubeconfigSamlTokenCleanup: deleting [%s]", token.Name)
					ch.tokens.Delete(token.Name, &metav1.DeleteOptions{})
				}
			}

		}
	}
}
