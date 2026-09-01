package client

const (
	GRPCActionType         = "grpcAction"
	GRPCActionFieldMode    = "mode"
	GRPCActionFieldPort    = "port"
	GRPCActionFieldService = "service"
)

type GRPCAction struct {
	Mode    string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Port    int64  `json:"port,omitempty" yaml:"port,omitempty"`
	Service string `json:"service,omitempty" yaml:"service,omitempty"`
}
