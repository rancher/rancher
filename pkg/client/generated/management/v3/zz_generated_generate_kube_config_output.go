package client


	


import (
	
)

const (
    GenerateKubeConfigOutputType = "generateKubeConfigOutput"
	GenerateKubeConfigOutputFieldConfig = "config"
)

type GenerateKubeConfigOutput struct {
        Config string `json:"config,omitempty" yaml:"config,omitempty"`
}

