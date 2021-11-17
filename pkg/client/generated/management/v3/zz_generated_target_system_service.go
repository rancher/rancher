package client


	


import (
	
)

const (
    TargetSystemServiceType = "targetSystemService"
	TargetSystemServiceFieldCondition = "condition"
)

type TargetSystemService struct {
        Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

