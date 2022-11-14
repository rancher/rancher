package models

type Nodepool struct {
	Quantity         int    `json:"quantity" yaml:"quantity"`
	Etcd             string `json:"etcd" yaml:"etcd"`
	Cp               string `json:"cp" yaml:"cp"`
	Wkr              string `json:"wkr" yaml:"wkr"`
	InstanceType     string `json:"instanceType" yaml:"instanceType"`
	DesiredSize      int    `json:"desiredSize" yaml:"desiredSize"`
	MaxSize          int    `json:"maxSize" yaml:"maxSize"`
	MinSize          int    `json:"minSize" yaml:"minSize"`
	MaxPodsContraint int    `json:"maxPodsContraint" yaml:"maxPodsContraint"`
}
