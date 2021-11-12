package client

const (
	GlobalAwsOptsType                             = "globalAwsOpts"
	GlobalAwsOptsFieldDisableSecurityGroupIngress = "disable-security-group-ingress"
	GlobalAwsOptsFieldDisableStrictZoneCheck      = "disable-strict-zone-check"
	GlobalAwsOptsFieldElbSecurityGroup            = "elb-security-group"
	GlobalAwsOptsFieldKubernetesClusterID         = "kubernetes-cluster-id"
	GlobalAwsOptsFieldKubernetesClusterTag        = "kubernetes-cluster-tag"
	GlobalAwsOptsFieldRoleARN                     = "role-arn"
	GlobalAwsOptsFieldRouteTableID                = "routetable-id"
	GlobalAwsOptsFieldSubnetID                    = "subnet-id"
	GlobalAwsOptsFieldVPC                         = "vpc"
	GlobalAwsOptsFieldZone                        = "zone"
)

type GlobalAwsOpts struct {
	DisableSecurityGroupIngress bool   `json:"disable-security-group-ingress,omitempty" yaml:"disable-security-group-ingress,omitempty"`
	DisableStrictZoneCheck      bool   `json:"disable-strict-zone-check,omitempty" yaml:"disable-strict-zone-check,omitempty"`
	ElbSecurityGroup            string `json:"elb-security-group,omitempty" yaml:"elb-security-group,omitempty"`
	KubernetesClusterID         string `json:"kubernetes-cluster-id,omitempty" yaml:"kubernetes-cluster-id,omitempty"`
	KubernetesClusterTag        string `json:"kubernetes-cluster-tag,omitempty" yaml:"kubernetes-cluster-tag,omitempty"`
	RoleARN                     string `json:"role-arn,omitempty" yaml:"role-arn,omitempty"`
	RouteTableID                string `json:"routetable-id,omitempty" yaml:"routetable-id,omitempty"`
	SubnetID                    string `json:"subnet-id,omitempty" yaml:"subnet-id,omitempty"`
	VPC                         string `json:"vpc,omitempty" yaml:"vpc,omitempty"`
	Zone                        string `json:"zone,omitempty" yaml:"zone,omitempty"`
}
