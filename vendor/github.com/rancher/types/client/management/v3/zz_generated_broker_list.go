package client

const (
	BrokerListType            = "brokerList"
	BrokerListFieldBrokerList = "brokerList"
)

type BrokerList struct {
	BrokerList []string `json:"brokerList,omitempty"`
}
