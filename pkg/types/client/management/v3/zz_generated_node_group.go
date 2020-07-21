package client

const (
	NodeGroupType                      = "nodeGroup"
	NodeGroupFieldDesiredSize          = "desiredSize"
	NodeGroupFieldDiskSize             = "diskSize"
	NodeGroupFieldEc2SshKey            = "ec2SshKey"
	NodeGroupFieldGpu                  = "gpu"
	NodeGroupFieldInstanceType         = "instanceType"
	NodeGroupFieldLabels               = "labels"
	NodeGroupFieldMaxSize              = "maxSize"
	NodeGroupFieldMinSize              = "minSize"
	NodeGroupFieldNodegroupName        = "nodegroupName"
	NodeGroupFieldSourceSecurityGroups = "sourceSecurityGroups"
	NodeGroupFieldSubnets              = "subnets"
	NodeGroupFieldTags                 = "tags"
	NodeGroupFieldVersion              = "version"
)

type NodeGroup struct {
	DesiredSize          *int64            `json:"desiredSize,omitempty" yaml:"desiredSize,omitempty"`
	DiskSize             *int64            `json:"diskSize,omitempty" yaml:"diskSize,omitempty"`
	Ec2SshKey            string            `json:"ec2SshKey,omitempty" yaml:"ec2SshKey,omitempty"`
	Gpu                  bool              `json:"gpu,omitempty" yaml:"gpu,omitempty"`
	InstanceType         string            `json:"instanceType,omitempty" yaml:"instanceType,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	MaxSize              *int64            `json:"maxSize,omitempty" yaml:"maxSize,omitempty"`
	MinSize              *int64            `json:"minSize,omitempty" yaml:"minSize,omitempty"`
	NodegroupName        string            `json:"nodegroupName,omitempty" yaml:"nodegroupName,omitempty"`
	SourceSecurityGroups []string          `json:"sourceSecurityGroups,omitempty" yaml:"sourceSecurityGroups,omitempty"`
	Subnets              []string          `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Version              string            `json:"version,omitempty" yaml:"version,omitempty"`
}
