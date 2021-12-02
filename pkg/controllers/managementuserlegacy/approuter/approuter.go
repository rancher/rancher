package approuter

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ingresswrapper"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	invalidRDNSServerBaseURL = "https://api.lb.rancher.cloud/v1"
)

// This controller is responsible for watching all the ingress resources in the cluster and creates the following DNS entries
// in <rancher_root_domain>:
// <$ingress_name>.<$namespace>.<$cluster_id>.<rancher_root_domain> => [ingress IPs]
// When an ingress resource has more than 10 IPs, only 10 IPs will be returned by DNS.
// In an RKE cluster, when a node becomes unhealthy and the corresponding nginx ingress resource becomes unavailable,
// the dynamic DNS controller updates the DNS mapping to remove that node IP from the list.
// Every once in a while (default 24h), the dynamic DNS controller will call renew to update the expiration time

func Register(ctx context.Context, cluster *config.UserContext) {
	starter := cluster.DeferredStart(ctx, func(ctx context.Context) error {
		registerDeferred(ctx, cluster)
		return nil
	})
	settingsController := cluster.Management.Management.Settings("")
	settingsController.AddHandler(ctx, "approuter-deferred",
		func(key string, setting *v3.Setting) (runtime.Object, error) {
			if settings.RDNSServerBaseURL.Get() != invalidRDNSServerBaseURL &&
				settings.RDNSServerBaseURL.Get() != "" {
				return setting, starter()
			}
			return setting, nil
		})
}

func registerDeferred(ctx context.Context, cluster *config.UserContext) {
	secrets := cluster.Management.Core.Secrets("")
	secretLister := cluster.Management.Core.Secrets("").Controller().Lister()
	workload := cluster.UserOnlyContext()
	c := &Controller{
		ingressInterface: ingresswrapper.NewCompatInterface(workload.Networking, workload.Extensions, workload.K8sClient),
		ingressLister:    ingresswrapper.NewCompatLister(workload.Networking, workload.Extensions, workload.K8sClient),
		dnsClient:        NewClient(secrets, secretLister, workload.ClusterName),
	}
	if c.ingressInterface.ServerSupportsIngressV1 {
		workload.Networking.Ingresses("").AddHandler(ctx, "approuterController", ingresswrapper.CompatSyncV1(c.sync))
	} else {
		workload.Extensions.Ingresses("").AddHandler(ctx, "approuterController", ingresswrapper.CompatSyncV1Beta1(c.sync))
	}
	go c.renew(ctx)
}
