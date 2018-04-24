package approuter

import (
	"context"

	workloadUtil "github.com/rancher/rancher/pkg/controllers/user/workload"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *config.UserContext) {
	secrets := cluster.Management.Core.Secrets("")
	secretLister := cluster.Management.Core.Secrets("").Controller().Lister()
	workload := cluster.UserOnlyContext()
	c := &Controller{
		ingressInterface:       workload.Extensions.Ingresses(""),
		ingressLister:          workload.Extensions.Ingresses("").Controller().Lister(),
		clusterName:            workload.ClusterName,
		managementSecretLister: secretLister,
	}
	workload.Extensions.Ingresses("").AddHandler("approuterController", c.sync)
	n := &NginxIngressController{
		podLister:         workload.Core.Pods(defaultNginxIngressNamespace).Controller().Lister(),
		nodeLister:        workload.Core.Nodes("").Controller().Lister(),
		clusterName:       workload.ClusterName,
		dnsClient:         NewClient(secrets, secretLister, workload.ClusterName),
		ingressController: workload.Extensions.Ingresses("").Controller(),
	}
	n.workloadController = workloadUtil.NewWorkloadController(workload, n.sync)
	go n.renew(ctx)
}
