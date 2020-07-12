package client

const (
	KafkaConfigType                    = "kafkaConfig"
	KafkaConfigFieldBrokerEndpoints    = "brokerEndpoints"
	KafkaConfigFieldCertificate        = "certificate"
	KafkaConfigFieldClientCert         = "clientCert"
	KafkaConfigFieldClientKey          = "clientKey"
	KafkaConfigFieldSaslPassword       = "saslPassword"
	KafkaConfigFieldSaslScramMechanism = "saslScramMechanism"
	KafkaConfigFieldSaslType           = "saslType"
	KafkaConfigFieldSaslUsername       = "saslUsername"
	KafkaConfigFieldTopic              = "topic"
	KafkaConfigFieldZookeeperEndpoint  = "zookeeperEndpoint"
)

type KafkaConfig struct {
	BrokerEndpoints    []string `json:"brokerEndpoints,omitempty" yaml:"brokerEndpoints,omitempty"`
	Certificate        string   `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientCert         string   `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey          string   `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	SaslPassword       string   `json:"saslPassword,omitempty" yaml:"saslPassword,omitempty"`
	SaslScramMechanism string   `json:"saslScramMechanism,omitempty" yaml:"saslScramMechanism,omitempty"`
	SaslType           string   `json:"saslType,omitempty" yaml:"saslType,omitempty"`
	SaslUsername       string   `json:"saslUsername,omitempty" yaml:"saslUsername,omitempty"`
	Topic              string   `json:"topic,omitempty" yaml:"topic,omitempty"`
	ZookeeperEndpoint  string   `json:"zookeeperEndpoint,omitempty" yaml:"zookeeperEndpoint,omitempty"`
}
