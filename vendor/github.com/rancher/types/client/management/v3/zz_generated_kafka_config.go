package client

const (
	KafkaConfigType                   = "kafkaConfig"
	KafkaConfigFieldBrokerEndpoints   = "brokerEndpoints"
	KafkaConfigFieldCertificate       = "certificate"
	KafkaConfigFieldClientCert        = "clientCert"
	KafkaConfigFieldClientKey         = "clientKey"
	KafkaConfigFieldTopic             = "topic"
	KafkaConfigFieldZookeeperEndpoint = "zookeeperEndpoint"
)

type KafkaConfig struct {
	BrokerEndpoints   []string `json:"brokerEndpoints,omitempty" yaml:"brokerEndpoints,omitempty"`
	Certificate       string   `json:"certificate,omitempty" yaml:"certificate,omitempty"`
	ClientCert        string   `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey         string   `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
	Topic             string   `json:"topic,omitempty" yaml:"topic,omitempty"`
	ZookeeperEndpoint string   `json:"zookeeperEndpoint,omitempty" yaml:"zookeeperEndpoint,omitempty"`
}
