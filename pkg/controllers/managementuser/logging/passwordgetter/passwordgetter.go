package passwordgetter

import (
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	passwordutil "github.com/rancher/rancher/pkg/api/norman/store/password"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
)

const (
	passwordSecretPrefix = "cattle-global-data:"
)

func NewPasswordGetter(secrets v1.SecretInterface) *PasswordGetter {
	return &PasswordGetter{
		secrets: secrets,
	}
}

type PasswordGetter struct {
	secrets v1.SecretInterface
}

func (p *PasswordGetter) GetPasswordFromSecret(loggingTarget *v32.LoggingTargets) (err error) {
	if loggingTarget.ElasticsearchConfig != nil && loggingTarget.ElasticsearchConfig.AuthPassword != "" && strings.HasPrefix(loggingTarget.ElasticsearchConfig.AuthPassword, passwordSecretPrefix) {
		if loggingTarget.ElasticsearchConfig.AuthPassword, err = passwordutil.GetValueForPasswordField(loggingTarget.ElasticsearchConfig.AuthPassword, p.secrets); err != nil {
			return
		}
	}

	if loggingTarget.SplunkConfig != nil && loggingTarget.SplunkConfig.Token != "" && strings.HasPrefix(loggingTarget.SplunkConfig.Token, passwordSecretPrefix) {
		if loggingTarget.SplunkConfig.Token, err = passwordutil.GetValueForPasswordField(loggingTarget.SplunkConfig.Token, p.secrets); err != nil {
			return
		}
	}

	if loggingTarget.KafkaConfig != nil && loggingTarget.KafkaConfig.SaslPassword != "" && strings.HasPrefix(loggingTarget.KafkaConfig.SaslPassword, passwordSecretPrefix) {
		if loggingTarget.KafkaConfig.SaslPassword, err = passwordutil.GetValueForPasswordField(loggingTarget.KafkaConfig.SaslPassword, p.secrets); err != nil {
			return
		}
	}

	if loggingTarget.SyslogConfig != nil && loggingTarget.SyslogConfig.Token != "" && strings.HasPrefix(loggingTarget.SyslogConfig.Token, passwordSecretPrefix) {
		if loggingTarget.SyslogConfig.Token, err = passwordutil.GetValueForPasswordField(loggingTarget.SyslogConfig.Token, p.secrets); err != nil {
			return
		}
	}

	if loggingTarget.FluentForwarderConfig != nil && len(loggingTarget.FluentForwarderConfig.FluentServers) != 0 {
		var newFluentdServers []v32.FluentServer
		for _, server := range loggingTarget.FluentForwarderConfig.FluentServers {
			newServer := server
			if server.SharedKey != "" && strings.HasPrefix(server.SharedKey, passwordSecretPrefix) {
				if newServer.SharedKey, err = passwordutil.GetValueForPasswordField(server.SharedKey, p.secrets); err != nil {
					return
				}
			}

			if server.Password != "" && strings.HasPrefix(server.Password, passwordSecretPrefix) {
				if newServer.Password, err = passwordutil.GetValueForPasswordField(server.Password, p.secrets); err != nil {
					return
				}
			}
			newFluentdServers = append(newFluentdServers, newServer)
		}
		loggingTarget.FluentForwarderConfig.FluentServers = newFluentdServers
	}

	return
}
