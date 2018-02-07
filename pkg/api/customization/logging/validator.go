package logging

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"

	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
)

func ClusterLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var clusterLogging v3.ClusterLoggingSpec
	if err := convert.ToObj(data, &clusterLogging); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	wp := utils.WrapClusterLogging{
		ClusterLoggingSpec: clusterLogging,
	}

	if err := wp.Validate(); err != nil {
		return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("%v", err))
	}
	return nil
}

func ProjectLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var projectLogging v3.ProjectLoggingSpec
	if err := convert.ToObj(data, &projectLogging); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	wp := utils.WrapProjectLogging{
		ProjectLoggingSpec: projectLogging,
	}

	if err := wp.Validate(); err != nil {
		return httperror.NewAPIError(httperror.InvalidFormat, fmt.Sprintf("%v", err))
	}
	return nil
}
