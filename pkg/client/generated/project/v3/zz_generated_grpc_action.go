package client

const (
	GRPCActionType         = "grpcAction"
	GRPCActionFieldPort    = "port"
	GRPCActionFieldService = "service"
)

type GRPCAction struct {
	Port    int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Service string `json:"service,omitempty" yaml:"service,omitempty"`
}
