package client

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	EndpointType                   = "endpoint"
	EndpointFieldBearerTokenSecret = "bearerTokenSecret"
	EndpointFieldHonorTimestamps   = "honorTimestamps"
	EndpointFieldInterval          = "interval"
	EndpointFieldParams            = "params"
	EndpointFieldPath              = "path"
	EndpointFieldRelabelConfigs    = "relabelings"
	EndpointFieldScheme            = "scheme"
	EndpointFieldScrapeTimeout     = "scrapeTimeout"
	EndpointFieldTargetPort        = "targetPort"
)

type Endpoint struct {
	BearerTokenSecret *SecretKeySelector  `json:"bearerTokenSecret,omitempty" yaml:"bearerTokenSecret,omitempty"`
	HonorTimestamps   *bool               `json:"honorTimestamps,omitempty" yaml:"honorTimestamps,omitempty"`
	Interval          string              `json:"interval,omitempty" yaml:"interval,omitempty"`
	Params            map[string][]string `json:"params,omitempty" yaml:"params,omitempty"`
	Path              string              `json:"path,omitempty" yaml:"path,omitempty"`
	RelabelConfigs    []RelabelConfig     `json:"relabelings,omitempty" yaml:"relabelings,omitempty"`
	Scheme            string              `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	ScrapeTimeout     string              `json:"scrapeTimeout,omitempty" yaml:"scrapeTimeout,omitempty"`
	TargetPort        intstr.IntOrString  `json:"targetPort,omitempty" yaml:"targetPort,omitempty"`
}
