package project

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/client/management/v3"
	"github.com/rancher/types/config"
)

func NewValidator(management *config.ScaledContext) types.Validator {
	validator := &Validator{
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}
	return validator.Validator
}

type Validator struct {
	clusterLister v3.ClusterLister
}

func (v *Validator) Validator(request *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	clusterId, ok := data[client.ProjectSpecFieldClusterId].(string)
	if !ok {
		return httperror.NewAPIError(httperror.InvalidBodyContent,
			fmt.Sprintf("no %v field exists or is parsable", client.ProjectSpecFieldClusterId))
	}

	cluster, err := v.clusterLister.Get("", clusterId)
	if err != nil {
		return httperror.NewAPIError(httperror.ServerError, fmt.Sprintf("error getting cluster: %v", err))
	}

	if cluster.Spec.DefaultPodSecurityPolicyTemplateName == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "cluster does not have PSPTs enabled")
	}

	return nil
}
