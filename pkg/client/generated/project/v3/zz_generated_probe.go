package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ProbeType                               = "probe"
	ProbeFieldCommand                       = "command"
	ProbeFieldFailureThreshold              = "failureThreshold"
	ProbeFieldGRPC                          = "grpc"
	ProbeFieldHTTPHeaders                   = "httpHeaders"
	ProbeFieldHost                          = "host"
	ProbeFieldInitialDelaySeconds           = "initialDelaySeconds"
	ProbeFieldPath                          = "path"
	ProbeFieldPeriodSeconds                 = "periodSeconds"
	ProbeFieldPort                          = "port"
	ProbeFieldScheme                        = "scheme"
	ProbeFieldSuccessThreshold              = "successThreshold"
	ProbeFieldTCP                           = "tcp"
	ProbeFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	ProbeFieldTimeoutSeconds                = "timeoutSeconds"
)

type Probe struct {
	Command                       []string           `json:"command,omitempty" yaml:"command,omitempty"`
	FailureThreshold              int64              `json:"failureThreshold,omitempty" yaml:"failureThreshold,omitempty"`
	GRPC                          *GRPCAction        `json:"grpc,omitempty" yaml:"grpc,omitempty"`
	HTTPHeaders                   []HTTPHeader       `json:"httpHeaders,omitempty" yaml:"httpHeaders,omitempty"`
	Host                          string             `json:"host,omitempty" yaml:"host,omitempty"`
	InitialDelaySeconds           int64              `json:"initialDelaySeconds,omitempty" yaml:"initialDelaySeconds,omitempty"`
	Path                          string             `json:"path,omitempty" yaml:"path,omitempty"`
	PeriodSeconds                 int64              `json:"periodSeconds,omitempty" yaml:"periodSeconds,omitempty"`
	Port                          intstr.IntOrString `json:"port,omitempty" yaml:"port,omitempty"`
	Scheme                        string             `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	SuccessThreshold              int64              `json:"successThreshold,omitempty" yaml:"successThreshold,omitempty"`
	TCP                           bool               `json:"tcp,omitempty" yaml:"tcp,omitempty"`
	TerminationGracePeriodSeconds *int64             `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	TimeoutSeconds                int64              `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}
