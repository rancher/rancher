package client


	


import (
	
)

const (
    FeatureSpecType = "featureSpec"
	FeatureSpecFieldValue = "value"
)

type FeatureSpec struct {
        Value *bool `json:"value,omitempty" yaml:"value,omitempty"`
}

