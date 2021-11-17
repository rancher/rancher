package client


	

	


import (
	
)

const (
    SupplementalGroupsStrategyOptionsType = "supplementalGroupsStrategyOptions"
	SupplementalGroupsStrategyOptionsFieldRanges = "ranges"
	SupplementalGroupsStrategyOptionsFieldRule = "rule"
)

type SupplementalGroupsStrategyOptions struct {
        Ranges []IDRange `json:"ranges,omitempty" yaml:"ranges,omitempty"`
        Rule string `json:"rule,omitempty" yaml:"rule,omitempty"`
}

