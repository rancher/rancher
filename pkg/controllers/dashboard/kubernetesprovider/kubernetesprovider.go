package kubernetesprovider

import (
	"context"
	"errors"

	detector "github.com/rancher/kubernetes-provider-detector"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"k8s.io/client-go/kubernetes"
)

const (
	ProviderKey = "provider.cattle.io"
)

type handler struct {
	ctx                context.Context
	clusters           mgmtcontrollers.ClusterClient
	localClusterClient kubernetes.Interface
	mcm                wrangler.MultiClusterManager
}

func Register(ctx context.Context,
	clusters mgmtcontrollers.ClusterController,
	localClusterClient kubernetes.Interface,
	mcm wrangler.MultiClusterManager,
) {
	h := &handler{
		ctx:                ctx,
		clusters:           clusters,
		localClusterClient: localClusterClient,
		mcm:                mcm,
	}
	clusters.OnChange(ctx, "kubernetes-provider", h.OnChange)
}

func (h *handler) OnChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil {
		return nil, nil
	}

	if cluster.Status.Provider != "" && cluster.Labels[ProviderKey] != "" {
		// if cluster has windows enabled and provider is not rke.windows, continue to detect if it has windows nodes
		if !cluster.Spec.WindowsPreferedCluster || cluster.Status.Provider == "rke.windows" {
			return cluster, nil
		}
	}

	// Downstream cluster must be reachable through client
	if !apimgmtv3.ClusterConditionProbeSucceeded.IsTrue(cluster) {
		return cluster, nil
	}

	var client kubernetes.Interface
	if cluster.Spec.Internal {
		client = h.localClusterClient
	} else if k8s, err := h.mcm.K8sClient(cluster.Name); err != nil {
		// ignore error. If we can't get a client just ignore it. The cluster probably isn't happy
		// yet and we will get an update later when it is.
		return nil, nil
	} else if k8s != nil {
		client = k8s
	}

	if client == nil {
		return cluster, nil
	}

	provider, err := detector.DetectProvider(h.ctx, client)
	if errors.Is(err, detector.ErrUnknownProvider) {
		return cluster, nil
	} else if err != nil {
		return cluster, err
	}

	cluster = cluster.DeepCopy()
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}

	if cluster.Labels[ProviderKey] != provider {
		cluster.Labels[ProviderKey] = provider
		cluster, err = h.clusters.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	if cluster.Status.Provider != provider {
		cluster.Status.Provider = provider
		return h.clusters.UpdateStatus(cluster)
	}

	return cluster, nil
}
