package approuter

import (
	"context"
	"github.com/rancher/types/config"
)

// This controller is responsible for watching all the ingress resources in the cluster and creates the following DNS entries
// in <rancher_root_domain>:
// <$ingress_name>.<$namespace>.<$cluster_id>.<rancher_root_domain> => [ingress IPs]
// When an ingress resource has more than 10 IPs, only 10 IPs will be returned by DNS.
// In an RKE cluster, when a node becomes unhealthy and the corresponding nginx ingress resource becomes unavailable,
// the dynamic DNS controller updates the DNS mapping to remove that node IP from the list.
// Every once in a while (default 24h), the dynamic DNS controller will call renew to update the expiration time

func Register(ctx context.Context, cluster *config.UserContext) {
	secrets := cluster.Management.Core.Secrets("")
	secretLister := cluster.Management.Core.Secrets("").Controller().Lister()
	workload := cluster.UserOnlyContext()
	c := &Controller{
		ingressInterface:       workload.Extensions.Ingresses(""),
		ingressLister:          workload.Extensions.Ingresses("").Controller().Lister(),
		dnsClient:              NewClient(secrets, secretLister, workload.ClusterName),
		clusterName:            workload.ClusterName,
		managementSecretLister: secretLister,
	}
	workload.Extensions.Ingresses("").AddHandler("approuterController", c.sync)
	go c.renew(ctx)
}
