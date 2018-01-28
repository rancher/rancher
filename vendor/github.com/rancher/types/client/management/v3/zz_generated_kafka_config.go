package client

const (
	KafkaConfigType                = "kafkaConfig"
	KafkaConfigFieldBroker         = "broker"
	KafkaConfigFieldDataType       = "dataType"
	KafkaConfigFieldMaxSendRetries = "maxSendRetries"
	KafkaConfigFieldTopic          = "topic"
	KafkaConfigFieldZookeeper      = "zookeeper"
)

type KafkaConfig struct {
	Broker         *BrokerList `json:"broker,omitempty"`
	DataType       string      `json:"dataType,omitempty"`
	MaxSendRetries *int64      `json:"maxSendRetries,omitempty"`
	Topic          string      `json:"topic,omitempty"`
	Zookeeper      *Zookeeper  `json:"zookeeper,omitempty"`
}
