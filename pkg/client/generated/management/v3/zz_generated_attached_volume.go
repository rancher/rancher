package client


	


import (
	
)

const (
    AttachedVolumeType = "attachedVolume"
	AttachedVolumeFieldName = "name"
)

type AttachedVolume struct {
        Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

