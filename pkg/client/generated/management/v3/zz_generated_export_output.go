package client


	


import (
	
)

const (
    ExportOutputType = "exportOutput"
	ExportOutputFieldYAMLOutput = "yamlOutput"
)

type ExportOutput struct {
        YAMLOutput string `json:"yamlOutput,omitempty" yaml:"yamlOutput,omitempty"`
}

