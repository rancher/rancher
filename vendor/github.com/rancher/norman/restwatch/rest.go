package restwatch

import (
	"time"

	"github.com/rancher/wrangler/pkg/ratelimit"
	"k8s.io/client-go/rest"
)

type WatchClient interface {
	WatchClient() rest.Interface
}

func UnversionedRESTClientFor(config *rest.Config) (rest.Interface, error) {
	// k8s <= 1.16 would not rate limit when calling UnversionedRESTClientFor(config)
	// this keeps that behavior which seems to be relied on in Rancher.
	if config.QPS == 0.0 && config.RateLimiter == nil {
		config.RateLimiter = ratelimit.None
	}
	client, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, err
	}

	if config.Timeout == 0 {
		return client, err
	}

	newConfig := *config
	newConfig.Timeout = 30 * time.Minute
	watchClient, err := rest.UnversionedRESTClientFor(&newConfig)
	if err != nil {
		return nil, err
	}

	return &clientWithWatch{
		RESTClient:  client,
		watchClient: watchClient,
	}, nil
}

type clientWithWatch struct {
	*rest.RESTClient
	watchClient *rest.RESTClient
}

func (c *clientWithWatch) WatchClient() rest.Interface {
	return c.watchClient
}
