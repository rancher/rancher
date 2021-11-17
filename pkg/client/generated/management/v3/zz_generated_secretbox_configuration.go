package client


	


import (
	
)

const (
    SecretboxConfigurationType = "secretboxConfiguration"
	SecretboxConfigurationFieldKeys = "keys"
)

type SecretboxConfiguration struct {
        Keys []Key `json:"keys,omitempty" yaml:"keys,omitempty"`
}

