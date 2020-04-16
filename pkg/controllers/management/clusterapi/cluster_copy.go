package clusterapi

import (
	"context"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/wrangler"
	apiv32 "github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io/v3"
	v12 "github.com/rancher/types/apis/core/v1"
	apiv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/flowcontrol"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
)

const (
	creatorIDAnn          = "field.cattle.io/creatorId"
	clusterAPIParentLabel = "cattle.io/clusterapi-parent"
)

type handler struct {
	RancherClusterCache      apiv32.ClusterCache
	RancherClusterClient     apiv32.ClusterClient
	RancherClusterController apiv3.ClusterController
	SecretLister             v12.SecretLister
	UserCache                apiv32.UserCache
	backoff                  *flowcontrol.Backoff
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext, manager *clustermanager.Manager) {
	h := &handler{
		RancherClusterCache:      wContext.Mgmt.Cluster().Cache(),
		RancherClusterClient:     wContext.Mgmt.Cluster(),
		RancherClusterController: mgmtCtx.Management.Clusters("").Controller(),
		SecretLister:             mgmtCtx.Core.Secrets("").Controller().Lister(),
		UserCache:                wContext.Mgmt.User().Cache(),
		backoff:                  flowcontrol.NewBackOff(30*time.Second, 10*time.Minute),
	}

	wContext.V1alpha3.Cluster().OnChange(ctx, "clusterapi-copier", h.onClusterChange)
}

func (h *handler) onClusterChange(key string, cluster *v1alpha3.Cluster) (*v1alpha3.Cluster, error) {
	if cluster == nil {
		return nil, nil
	}

	matchingClusters, err := h.RancherClusterClient.List(v1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", clusterAPIParentLabel, cluster.Name)})
	if err != nil {
		return cluster, err
	}

	if cluster.DeletionTimestamp != nil {
		if len(matchingClusters.Items) == 0 {
			return nil, nil
		}
		// not using owner reference because the rancher cluster should be deleted right away to match
		// rancher behavior with regular clusters. Otherwise, cluster will stay up but lose communication
		// as clusterapi deletes infrastructure.
		return nil, h.RancherClusterClient.Delete(matchingClusters.Items[0].Name, &v1.DeleteOptions{})
	}

	if cluster.Status.GetTypedPhase() == v1alpha3.ClusterPhaseDeleting {
		fmt.Println("deleting but didn't delete rancher cluster")
	}

	var rancherCluster *apiv3.Cluster
	if len(matchingClusters.Items) == 0 {
		creatorName, err := h.getBootstrapAdminName()
		if err != nil {
			return cluster, err
		}

		rancherCluster = &apiv3.Cluster{
			ObjectMeta: v1.ObjectMeta{
				GenerateName: "c-",
				Labels: map[string]string{
					"cattle.io/clusterapi-parent": cluster.Name,
				},
				Annotations: map[string]string{
					creatorIDAnn: creatorName,
				},
			},
			Spec: apiv3.ClusterSpec{
				DisplayName:      cluster.Name,
				ClusterAPIConfig: cluster.Name,
			},
			Status: apiv3.ClusterStatus{
				Driver: "clusterAPI",
			},
		}
		rancherCluster, err = h.RancherClusterClient.Create(rancherCluster)
		if err != nil {
			return cluster, err
		}
	}

	return cluster, nil
}
func (h *handler) getBootstrapAdminName() (string, error) {
	cannotFindAdminErr := fmt.Errorf("unable to find bootstrap admin")

	selector := labels.NewSelector()
	adminLabel, err := labels.NewRequirement("authz.management.cattle.io/bootstrapping", selection.Equals, []string{"admin-user"})
	selector = selector.Add(*adminLabel)
	admins, err := h.UserCache.List(selector)
	if err != nil {
		return "", cannotFindAdminErr
	}

	if len(admins) == 0 {
		return "", cannotFindAdminErr
	}

	return admins[0].Name, nil
}
