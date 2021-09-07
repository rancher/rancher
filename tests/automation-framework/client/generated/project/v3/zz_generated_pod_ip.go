package client

const (
	PodIPType    = "podIP"
	PodIPFieldIP = "ip"
)

type PodIP struct {
	IP string `json:"ip,omitempty" yaml:"ip,omitempty"`
}
