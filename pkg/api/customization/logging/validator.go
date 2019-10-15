package logging

import (
	"fmt"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/generator"
	loggingutils "github.com/rancher/rancher/pkg/controllers/user/logging/utils"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
)

func ClusterLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec v3.ClusterLoggingSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	return validate(loggingconfig.ClusterLevel, "cluster", spec.LoggingTargets, spec.OutputTags)
}

func ProjectLoggingValidator(resquest *types.APIContext, schema *types.Schema, data map[string]interface{}) error {
	var spec v3.ProjectLoggingSpec
	if err := convert.ToObj(data, &spec); err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, fmt.Sprintf("%v", err))
	}

	return validate(loggingconfig.ProjectLevel, spec.ProjectName, spec.LoggingTargets, spec.OutputTags)
}

func validate(level, containerLogSourceTag string, loggingTargets v3.LoggingTargets, outputTags map[string]string) error {
	if loggingTargets.KafkaConfig != nil {
		if err := validateKafka(loggingTargets.KafkaConfig); err != nil {
			return err
		}
	}

	wrapTarget, err := generator.NewLoggingTargetTemplateWrap(loggingTargets)
	if err != nil {
		return err
	}

	if wrapTarget == nil {
		return nil
	}

	loggingCommomFileds := v3.LoggingCommonField{
		OutputTags: outputTags,
	}

	if loggingTargets.FluentForwarderConfig != nil && wrapTarget.EnableShareKey {
		wrapTarget.EnableShareKey = false //skip generate precan configure included ruby code
	}

	var wrap interface{}
	if level == loggingconfig.ProjectLevel {
		wrap = generator.ProjectLoggingTemplateWrap{
			ContainerLogSourceTag:     containerLogSourceTag,
			LoggingTargetTemplateWrap: *wrapTarget,
			LoggingCommonField:        loggingCommomFileds,
		}
	} else {
		wrap = generator.ClusterLoggingTemplateWrap{
			ContainerLogSourceTag:     containerLogSourceTag,
			LoggingTargetTemplateWrap: *wrapTarget,
			LoggingCommonField:        loggingCommomFileds,
		}
	}

	if loggingTargets.SyslogConfig != nil && loggingTargets.SyslogConfig.Token != "" {
		if err = generator.ValidateSyslogToken(wrap); err != nil {
			return err
		}
	}

	if len(outputTags) != 0 {
		if err = generator.ValidateCustomTags(wrap); err != nil {
			return err
		}
	}

	return generator.ValidateCustomTarget(wrap)
}

func validateKafka(kafkaConfig *v3.KafkaConfig) error {
	if kafkaConfig.SaslType == "plain" && kafkaConfig.ClientCert == "" && kafkaConfig.ClientKey == "" {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "Plain SASL authentication requires SSL is configured")
	}

	isSelfSigned, err := loggingutils.IsSelfSigned([]byte(kafkaConfig.Certificate))
	if err != nil {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "parse certificate failed")
	}

	if loggingutils.IsClientAuthEnaled(kafkaConfig.ClientCert, kafkaConfig.ClientKey) && isSelfSigned {
		return httperror.NewAPIError(httperror.InvalidBodyContent, "Certificate verification failed, Kafka doesn't support self-signed certificate when client authentication is enabled")
	}
	return nil
}
