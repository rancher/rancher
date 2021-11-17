package client


	

	


import (
	
)

const (
    IngressCapabilitiesType = "ingressCapabilities"
	IngressCapabilitiesFieldCustomDefaultBackend = "customDefaultBackend"
	IngressCapabilitiesFieldIngressProvider = "ingressProvider"
)

type IngressCapabilities struct {
        CustomDefaultBackend *bool `json:"customDefaultBackend,omitempty" yaml:"customDefaultBackend,omitempty"`
        IngressProvider string `json:"ingressProvider,omitempty" yaml:"ingressProvider,omitempty"`
}

