package kontainerdriver

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/multiclustermanager/whitelist"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

func Register(ctx context.Context, management *config.ScaledContext) {
	management.Management.KontainerDrivers("").AddHandler(ctx, "whitelist-proxy", sync)
}

func sync(key string, kontainerDriver *v3.KontainerDriver) (runtime.Object, error) {
	if key == "" || kontainerDriver == nil {
		return nil, nil
	}
	if kontainerDriver.DeletionTimestamp != nil {
		whitelist.Proxy.RmSource(string(kontainerDriver.UID))
		return nil, nil
	}

	whitelist.Proxy.RmSource(string(kontainerDriver.UID))
	for _, d := range kontainerDriver.Spec.WhitelistDomains {
		err := whitelist.Proxy.Add(d, string(kontainerDriver.UID))
		if err != nil {
			logrus.Debugf("failed to add domain %s to proxy accept list: %v", d, err)
		}
	}
	return nil, nil
}
