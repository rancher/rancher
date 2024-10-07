package configmaps

import (
	"fmt"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/configmaps"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	coreV1 "k8s.io/api/core/v1"
	"github.com/rancher/shepherd/pkg/wrangler"
)

// CreateConfigmap is a helper to create a configmap using public API.
func CreateConfigmap(namespace string, client *rancher.Client, data map[string]string, clusterID string) (configMap *coreV1.ConfigMap, err error) {
	var ctx *wrangler.Context
	
	if clusterID == "local" {
		ctx = client.WranglerContext
	} else {
		ctx, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return nil, fmt.Errorf("failed to get downstream context: %w", err)
		}
	}
	

	newConfigmap := configmaps.NewConfigmapTemplate(namegen.AppendRandomString("testcm-"), namespace, nil, nil, data)
	configMap, err = ctx.Core.ConfigMap().Create(&newConfigmap)

	return configMap, err
}
