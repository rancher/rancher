package clusterstatus

import (
	"context"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime"

	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

const TemporaryCredentialsAnnotationKey = "clusterstatus.management.cattle.io/temporary-security-credentials"

func Register(ctx context.Context, management *config.ManagementContext) {
	c := &clusterAnnotations{
		clusters: management.Management.Clusters(""),
	}

	management.Management.Clusters("").AddHandler(ctx, "temporary-credentials", c.sync)
}

type clusterAnnotations struct {
	clusters v3.ClusterInterface
}

func (cd *clusterAnnotations) sync(key string, cluster *v3.Cluster) (runtime.Object, error) {
	if key == "" || cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	if genericConfig := cluster.Spec.GenericEngineConfig; genericConfig != nil {
		eksConfig := *genericConfig
		if eksConfig["driverName"] != "amazonelasticcontainerservice" {
			return nil, nil
		}

		newValue := strconv.FormatBool(eksConfig["sessionToken"] != "" && eksConfig["sessionToken"] != nil)
		original := cluster
		cluster = original.DeepCopy()

		if cluster.Annotations == nil {
			cluster.Annotations = make(map[string]string)
		}

		cluster.Annotations[TemporaryCredentialsAnnotationKey] = newValue
		_, err := cd.clusters.Update(cluster)
		if err != nil {
			return nil, fmt.Errorf("error updating temporary credentials annotation: %v", err)
		}
	}

	return nil, nil
}
