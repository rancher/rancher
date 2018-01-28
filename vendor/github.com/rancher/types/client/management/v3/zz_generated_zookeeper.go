package client

const (
	ZookeeperType      = "zookeeper"
	ZookeeperFieldHost = "host"
	ZookeeperFieldPort = "port"
)

type Zookeeper struct {
	Host string `json:"host,omitempty"`
	Port *int64 `json:"port,omitempty"`
}
