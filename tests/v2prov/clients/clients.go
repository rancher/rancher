package clients

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/ratelimit"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

type Clients struct {
	*wrangler.Context
	Dynamic dynamic.Interface

	// Ctx is canceled when the Close() is called
	Ctx     context.Context
	cancel  func()
	onClose []func()
}

func (c *Clients) Close() {
	for i := len(c.onClose); i > 0; i-- {
		c.onClose[i-1]()
	}
	c.cancel()
}

func (c *Clients) OnClose(f func()) {
	c.onClose = append(c.onClose, f)
}

func (c *Clients) ForCluster(namespace, name string) (*Clients, error) {
	secret, err := c.Core.Secret().Get(namespace, name+"-kubeconfig", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.NewClientConfigFromBytes(secret.Data["value"])
	if err != nil {
		return nil, err
	}

	return NewForConfig(c.Ctx, config)
}

func New() (*Clients, error) {
	config := kubeconfig.GetNonInteractiveClientConfig("")
	return NewForConfig(context.Background(), config)
}

func NewForConfig(ctx context.Context, config clientcmd.ClientConfig) (*Clients, error) {
	ctx, cancel := context.WithCancel(ctx)

	rest, err := config.ClientConfig()
	if err != nil {
		cancel()
		return nil, err
	}

	rest.Timeout = 30 * time.Minute
	rest.RateLimiter = ratelimit.None

	wranglerCtx, err := wrangler.NewContext(ctx, config, rest)
	if err != nil {
		cancel()
		return nil, err
	}

	dynamic, err := dynamic.NewForConfig(rest)
	if err != nil {
		cancel()
		return nil, err
	}

	return &Clients{
		Context: wranglerCtx,
		Dynamic: dynamic,
		Ctx:     ctx,
		cancel:  cancel,
	}, nil
}
