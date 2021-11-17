package client


	


import (
	
)

const (
    WeaveNetworkProviderType = "weaveNetworkProvider"
	WeaveNetworkProviderFieldPassword = "password"
)

type WeaveNetworkProvider struct {
        Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

