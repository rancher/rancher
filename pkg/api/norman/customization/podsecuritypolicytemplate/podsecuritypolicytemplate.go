package podsecuritypolicytemplate

import (
	"github.com/rancher/norman/types"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	schema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/cache"
)

type Store struct {
	types.Store
}

const clusterByPSPTKey = "clusterByPSPT"
const projectByPSPTKey = "projectByPSPT"

func RegisterIndexers(config *wrangler.Context) {
	config.Mgmt.Cluster().Cache().AddIndexer(clusterByPSPTKey, clusterByPSPT)
}

func clusterByPSPT(cluster *v3.Cluster) ([]string, error) {
	return []string{cluster.Spec.DefaultPodSecurityPolicyTemplateName}, nil
}

type Format struct {
	ClusterIndexer cache.Indexer
	ProjectIndexer cache.Indexer
}

func (f *Format) Formatter(apiContext *types.APIContext, resource *types.RawResource) {
	// check if PSPT is assigned to a cluster or project
	projectsWithPSPT, err := f.ProjectIndexer.ByIndex(projectByPSPTKey, resource.ID)
	if err != nil {
		logrus.Warnf("failed to determine if PSPT was assigned to a project: %v", err)
		return
	}

	if len(projectsWithPSPT) != 0 {
		// remove delete link
		delete(resource.Links, "remove")
		return
	}

	clustersWithPSPT, err := f.ClusterIndexer.ByIndex(clusterByPSPTKey, resource.ID)
	if err != nil {
		logrus.Warnf("failed to determine if a PSPT was assigned to a cluster: %v", err)
		return
	}

	if len(clustersWithPSPT) != 0 {
		// remove delete link
		delete(resource.Links, "remove")
		return
	}
}

func clusterHasPSPTAssigned(apiContext *types.APIContext) (bool, error) {
	clusterSchema := apiContext.Schemas.Schema(&schema.Version, client.ClusterType)
	clusters, err := clusterSchema.Store.List(apiContext, clusterSchema, &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString(client.ClusterFieldDefaultPodSecurityPolicyTemplateID, types.ModifierEQ,
				apiContext.ID),
		},
	})
	return len(clusters) != 0, err
}
