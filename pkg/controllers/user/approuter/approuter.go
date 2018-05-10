package approuter

import (
	"context"
	"github.com/rancher/types/config"
)

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
