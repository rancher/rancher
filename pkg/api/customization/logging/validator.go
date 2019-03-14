package logging

import (
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

func ClusterLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec v3.ClusterLoggingSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	if spec.KafkaConfig != nil {
		return validateKafka(spec.KafkaConfig)
	}

	if err := validateBadConfig(spec.OutputTags, spec.CustomTargetConfig); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	return nil
}

func ProjectLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec v3.ProjectLoggingSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}

	if spec.KafkaConfig != nil {
		return validateKafka(spec.KafkaConfig)
	}

	if err := validateBadConfig(spec.OutputTags, spec.CustomTargetConfig); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, err.Error())
	}
	return nil
}

func validateKafka(kafkaConfig *v3.KafkaConfig) error {
	if kafkaConfig.SaslType == "plain" && kafkaConfig.ClientCert == "" && kafkaConfig.ClientKey == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "Plain SASL authentication requires SSL is configured")
	}
	return nil
}

func validateBadConfig(outputTags map[string]string, customTargetConfig *v3.CustomTargetConfig) error {
	if outputTags != nil {
		if _, err := utils.ValidateCustomTags(outputTags, false); err != nil {
			return err
		}
	}

	if customTargetConfig != nil {
		if err := utils.ValidateCustomTargetContent(customTargetConfig.Content); err != nil {
			return err
		}
	}
	return nil
}
