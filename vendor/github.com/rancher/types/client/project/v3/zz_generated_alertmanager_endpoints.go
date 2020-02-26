package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	AlertmanagerEndpointsType                 = "alertmanagerEndpoints"
	AlertmanagerEndpointsFieldAPIVersion      = "apiVersion"
	AlertmanagerEndpointsFieldBearerTokenFile = "bearerTokenFile"
	AlertmanagerEndpointsFieldName            = "name"
	AlertmanagerEndpointsFieldNamespace       = "namespace"
	AlertmanagerEndpointsFieldPathPrefix      = "pathPrefix"
	AlertmanagerEndpointsFieldPort            = "port"
	AlertmanagerEndpointsFieldScheme          = "scheme"
	AlertmanagerEndpointsFieldTLSConfig       = "tlsConfig"
)

type AlertmanagerEndpoints struct {
	APIVersion      string             `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	BearerTokenFile string             `json:"bearerTokenFile,omitempty" yaml:"bearerTokenFile,omitempty"`
	Name            string             `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace       string             `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	PathPrefix      string             `json:"pathPrefix,omitempty" yaml:"pathPrefix,omitempty"`
	Port            intstr.IntOrString `json:"port,omitempty" yaml:"port,omitempty"`
	Scheme          string             `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	TLSConfig       *TLSConfig         `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
}
