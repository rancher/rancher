package logging

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func ClusterLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec v3.ClusterLoggingSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	if spec.KafkaConfig != nil {
		return validateKafka(spec.KafkaConfig)
	}

	return nil
}

func ProjectLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec v3.ProjectLoggingSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	if spec.KafkaConfig != nil {
		return validateKafka(spec.KafkaConfig)
	}

	return nil
}

func validateKafka(kafkaConfig *v3.KafkaConfig) error {
	if kafkaConfig.SaslType == "plain" && kafkaConfig.ClientCert == "" && kafkaConfig.ClientKey == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "Plain SASL authentication requires SSL is configured")
	}
	return nil
}
