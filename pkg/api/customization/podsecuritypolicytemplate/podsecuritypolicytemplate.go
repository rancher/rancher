package podsecuritypolicytemplate

import (
	"fmt"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3/schema"
	client "github.com/rancher/rancher/pkg/types/client/management/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

type Store struct {
	types.Store
}

func (s *Store) Delete(apiContext *types.APIContext, schema *types.Schema, id string) (map[string]interface{}, error) {
	projectHasPSPT, err := projectHasPSPTAssigned(apiContext)
	if err != nil {
		return nil, fmt.Errorf("error checking if PSPT is assigned to projects: %v", err)
	}

	if projectHasPSPT {
		return nil, errors.NewBadRequest("PSPT is assigned to one or more projects, remove PSPT from those " +
			"projects before deleting")
	}

	clusterHasPSPT, err := clusterHasPSPTAssigned(apiContext)
	if err != nil {
		return nil, fmt.Errorf("error checking if PSPT is assigned to clusters: %v", err)
	}

	if clusterHasPSPT {
		return nil, errors.NewBadRequest("PSPT is assigned to one or more clusters, remove PSPT from those " +
			"clusters before deleting")
	}

	return s.Store.Delete(apiContext, schema, id)
}

const clusterByPSPTKey = "clusterByPSPT"
const projectByPSPTKey = "projectByPSPT"

func NewFormatter(management *config.ScaledContext) types.Formatter {
	clusterInformer := management.Management.Clusters("").Controller().Informer()
	clusterInformer.AddIndexers(map[string]cache.IndexFunc{
		clusterByPSPTKey: clusterByPSPT,
	})

	projectInformer := management.Management.Projects("").Controller().Informer()
	projectInformer.AddIndexers(map[string]cache.IndexFunc{
		projectByPSPTKey: projectByPSPT,
	})

	format := Format{
		ClusterIndexer: clusterInformer.GetIndexer(),
		ProjectIndexer: projectInformer.GetIndexer(),
	}
	return format.Formatter
}

func clusterByPSPT(obj interface{}) ([]string, error) {
	cluster, ok := obj.(*v3.Cluster)
	if !ok {
		return []string{}, nil
	}

	return []string{cluster.Spec.DefaultPodSecurityPolicyTemplateName}, nil
}

func projectByPSPT(obj interface{}) ([]string, error) {
	project, ok := obj.(*v3.Project)
	if !ok {
		return []string{}, nil
	}

	return []string{project.Status.PodSecurityPolicyTemplateName}, nil
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

func projectHasPSPTAssigned(apiContext *types.APIContext) (bool, error) {
	projectSchema := apiContext.Schemas.Schema(&schema.Version, client.ProjectType)
	projects, err := projectSchema.Store.List(apiContext, projectSchema, &types.QueryOptions{
		Conditions: []*types.QueryCondition{
			types.NewConditionFromString(client.ProjectFieldPodSecurityPolicyTemplateName, types.ModifierEQ,
				apiContext.ID),
		},
	})
	return len(projects) != 0, err
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
