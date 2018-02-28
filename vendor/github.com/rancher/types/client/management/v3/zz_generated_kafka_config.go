package client

const (
	KafkaConfigType                   = "kafkaConfig"
	KafkaConfigFieldBrokerEndpoints   = "brokerEndpoints"
	KafkaConfigFieldTopic             = "topic"
	KafkaConfigFieldZookeeperEndpoint = "zookeeperEndpoint"
)

type KafkaConfig struct {
	BrokerEndpoints   []string `json:"brokerEndpoints,omitempty" yaml:"brokerEndpoints,omitempty"`
	Topic             string   `json:"topic,omitempty" yaml:"topic,omitempty"`
	ZookeeperEndpoint string   `json:"zookeeperEndpoint,omitempty" yaml:"zookeeperEndpoint,omitempty"`
}
