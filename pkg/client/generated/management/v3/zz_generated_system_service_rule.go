package client


	


import (
	
)

const (
    SystemServiceRuleType = "systemServiceRule"
	SystemServiceRuleFieldCondition = "condition"
)

type SystemServiceRule struct {
        Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
}

