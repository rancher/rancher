package client

const (
	NodeGroupType                      = "nodeGroup"
	NodeGroupFieldArm                  = "arm"
	NodeGroupFieldDesiredSize          = "desiredSize"
	NodeGroupFieldDiskSize             = "diskSize"
	NodeGroupFieldEc2SshKey            = "ec2SshKey"
	NodeGroupFieldGpu                  = "gpu"
	NodeGroupFieldImageID              = "imageId"
	NodeGroupFieldInstanceType         = "instanceType"
	NodeGroupFieldLabels               = "labels"
	NodeGroupFieldLaunchTemplate       = "launchTemplate"
	NodeGroupFieldMaxSize              = "maxSize"
	NodeGroupFieldMinSize              = "minSize"
	NodeGroupFieldNodeRole             = "nodeRole"
	NodeGroupFieldNodegroupName        = "nodegroupName"
	NodeGroupFieldRequestSpotInstances = "requestSpotInstances"
	NodeGroupFieldResourceTags         = "resourceTags"
	NodeGroupFieldSpotInstanceTypes    = "spotInstanceTypes"
	NodeGroupFieldSubnets              = "subnets"
	NodeGroupFieldTags                 = "tags"
	NodeGroupFieldUserData             = "userData"
	NodeGroupFieldVersion              = "version"
)

type NodeGroup struct {
	Arm                  *bool             `json:"arm,omitempty" yaml:"arm,omitempty"`
	DesiredSize          *int64            `json:"desiredSize,omitempty" yaml:"desiredSize,omitempty"`
	DiskSize             *int64            `json:"diskSize,omitempty" yaml:"diskSize,omitempty"`
	Ec2SshKey            *string           `json:"ec2SshKey,omitempty" yaml:"ec2SshKey,omitempty"`
	Gpu                  *bool             `json:"gpu,omitempty" yaml:"gpu,omitempty"`
	ImageID              *string           `json:"imageId,omitempty" yaml:"imageId,omitempty"`
	InstanceType         *string           `json:"instanceType,omitempty" yaml:"instanceType,omitempty"`
	Labels               map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LaunchTemplate       *LaunchTemplate   `json:"launchTemplate,omitempty" yaml:"launchTemplate,omitempty"`
	MaxSize              *int64            `json:"maxSize,omitempty" yaml:"maxSize,omitempty"`
	MinSize              *int64            `json:"minSize,omitempty" yaml:"minSize,omitempty"`
	NodeRole             *string           `json:"nodeRole,omitempty" yaml:"nodeRole,omitempty"`
	NodegroupName        *string           `json:"nodegroupName,omitempty" yaml:"nodegroupName,omitempty"`
	RequestSpotInstances *bool             `json:"requestSpotInstances,omitempty" yaml:"requestSpotInstances,omitempty"`
	ResourceTags         map[string]string `json:"resourceTags,omitempty" yaml:"resourceTags,omitempty"`
	SpotInstanceTypes    []string          `json:"spotInstanceTypes,omitempty" yaml:"spotInstanceTypes,omitempty"`
	Subnets              []string          `json:"subnets,omitempty" yaml:"subnets,omitempty"`
	Tags                 map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
	UserData             *string           `json:"userData,omitempty" yaml:"userData,omitempty"`
	Version              *string           `json:"version,omitempty" yaml:"version,omitempty"`
}
