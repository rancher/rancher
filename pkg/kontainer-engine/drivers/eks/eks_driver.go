package eks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/iam"
	heptio "github.com/heptio/authenticator/pkg/token"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/options"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/util"
	"github.com/rancher/rancher/pkg/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	amiNamePrefix = "amazon-eks-node-"
)

var amiForRegionAndVersion = map[string]map[string]string{
	"1.23": {
		"us-east-2":      "ami-097a0fee8b900f064",
		"us-east-1":      "ami-087b755b20f3ec965",
		"us-west-2":      "ami-05842abec80265d2f",
		"us-west-1":      "ami-0d04dab984be47527",
		"af-south-1":     "ami-0b72eb3b172f69e29",
		"ap-east-1":      "ami-0d0127ee95766f431",
		"ap-south-1":     "ami-0140882fe82842c10",
		"ap-northeast-1": "ami-071e74b0e9b9b764b",
		"ap-northeast-2": "ami-0fa9bd99d9b0761b9",
		"ap-northeast-3": "ami-01a17775888a92dd6",
		"ap-southeast-1": "ami-041d9e0ee63520d26",
		"ap-southeast-2": "ami-056f9a46907fa044b",
		"ca-central-1":   "ami-0f699ffc0ad1048b5",
		"cn-north-1":     "ami-03c283d37d6d555c9",
		"cn-northwest-1": "ami-052f05a1bb39ebae2",
		"eu-central-1":   "ami-01c52a64630ff492f",
		"eu-west-1":      "ami-044d355a56926f0c6",
		"eu-west-2":      "ami-045ff0c109e41462b",
		"eu-south-1":     "ami-0df107f6c0af1908e",
		"eu-west-3":      "ami-0c60da043915b71d8",
		"eu-north-1":     "ami-031e7d17763317e60",
		"me-south-1":     "ami-00dd8fe486ecab5dd",
		"sa-east-1":      "ami-07556fcfb1932dd5c",
	},
	"1.22": {
		"us-east-2":      "ami-07b7136d362146ad8",
		"us-east-1":      "ami-06ce408c7d97c5f2d",
		"us-west-2":      "ami-014a7e726336f4390",
		"us-west-1":      "ami-0d82cc82431a571ca",
		"af-south-1":     "ami-0eda36353e4c11e10",
		"ap-east-1":      "ami-01e5761047a95ebae",
		"ap-south-1":     "ami-0bb79b20b8352e60b",
		"ap-northeast-1": "ami-038712d074ace1313",
		"ap-northeast-2": "ami-03f5c5b7ba2015719",
		"ap-northeast-3": "ami-006577a6173ae6031",
		"ap-southeast-1": "ami-0ed3ad8c0eaedbc28",
		"ap-southeast-2": "ami-01fd03b140a7aad21",
		"ca-central-1":   "ami-027c3a3edf4373ae6",
		"cn-north-1":     "ami-0078eb8f790bb1c73",
		"cn-northwest-1": "ami-0e863f0d49a4719d1",
		"eu-central-1":   "ami-068d000e86e5d6a81",
		"eu-west-1":      "ami-0ec9e1727a24fb788",
		"eu-west-2":      "ami-076c1675dca5420a3",
		"eu-south-1":     "ami-07786e9b9809131e5",
		"eu-west-3":      "ami-0661f3ccf87444044",
		"eu-north-1":     "ami-077d54d7249c31188",
		"me-south-1":     "ami-07d30ab2ef099490c",
		"sa-east-1":      "ami-0d00214b2d1aef170",
	},
	"1.21": {
		"us-east-2":      "ami-082ccdd64b87928ba",
		"us-east-1":      "ami-0d78302dd24db83c7",
		"us-west-2":      "ami-092cc88f5f93a14d3",
		"us-west-1":      "ami-035198547a41c99c8",
		"af-south-1":     "ami-00f3190abaddf36f1",
		"ap-east-1":      "ami-0bbe1f524f9932a15",
		"ap-south-1":     "ami-0d99e3aaf18224c52",
		"ap-northeast-1": "ami-0850351df9083199d",
		"ap-northeast-2": "ami-0e155c8c5a16e76f9",
		"ap-northeast-3": "ami-0ee08adfb314eaed2",
		"ap-southeast-1": "ami-0c9d3234317a56614",
		"ap-southeast-2": "ami-00fcd54e2ac6970a3",
		"ca-central-1":   "ami-0ff4205328ef2c72f",
		"cn-north-1":     "ami-0a8893e2be26e0b30",
		"cn-northwest-1": "ami-0400b7c06f0b0dc7e",
		"eu-central-1":   "ami-0241d3166f8e974f0",
		"eu-west-1":      "ami-0c94ad4ea417b5afb",
		"eu-west-2":      "ami-01218d75a193762c1",
		"eu-south-1":     "ami-06b634339bfed4b26",
		"eu-west-3":      "ami-03a643549ec59d1d5",
		"eu-north-1":     "ami-020e9b2d07d73eff4",
		"me-south-1":     "ami-02571be59f7a02d7f",
		"sa-east-1":      "ami-0a114e078e20d24ea",
	},
	"1.20": {
		"us-east-2":      "ami-0f93bf6b2b9569c78",
		"us-east-1":      "ami-0f9112de1cc4579fb",
		"us-west-2":      "ami-01a2cfbadc91647c7",
		"us-west-1":      "ami-016ff65935e7735ba",
		"af-south-1":     "ami-08c1897e371206237",
		"ap-east-1":      "ami-012a3027ee75de099",
		"ap-south-1":     "ami-02c38f5383509f61b",
		"ap-northeast-1": "ami-0f77fcce443d1521c",
		"ap-northeast-2": "ami-0c289eaf297babca4",
		"ap-northeast-3": "ami-0a116cfe84f55c117",
		"ap-southeast-1": "ami-0d182bb62e911c3a8",
		"ap-southeast-2": "ami-084cf31ad9678bd51",
		"ca-central-1":   "ami-00052a30a0a50f2e6",
		"cn-north-1":     "ami-06713bbde58657a46",
		"cn-northwest-1": "ami-0a212aae8ecbda546",
		"eu-central-1":   "ami-0c4f99474a0af6a06",
		"eu-west-1":      "ami-00a66cafe286616b5",
		"eu-west-2":      "ami-0d23be04329798f2b",
		"eu-south-1":     "ami-0cc934f53857e40c1",
		"eu-west-3":      "ami-001bf06a661fc1747",
		"eu-north-1":     "ami-0b81a4beabfba1444",
		"me-south-1":     "ami-024310edca5165069",
		"sa-east-1":      "ami-09315f5beb5261d6d",
	},
	"1.19": {
		"us-east-2":      "ami-098bdf416cda68d0d",
		"us-east-1":      "ami-0ce0eb4ea460ab7bf",
		"us-west-2":      "ami-0b92e1797a6aaa39f",
		"us-west-1":      "ami-0d5a0c0d43ec2ba81",
		"af-south-1":     "ami-082d761435204d831",
		"ap-east-1":      "ami-0e8230a549493f395",
		"ap-south-1":     "ami-0fba42baaa11f548a",
		"ap-northeast-1": "ami-095cb3f3f7baaea6d",
		"ap-northeast-2": "ami-0d806eabd1d9c25c4",
		"ap-northeast-3": "ami-0e06f42f1b8a189ad",
		"ap-southeast-1": "ami-08031e643c52754bf",
		"ap-southeast-2": "ami-0129a8d04bcea3e22",
		"ca-central-1":   "ami-095ce6f7ecec9b170",
		"cn-north-1":     "ami-065dff7849a61eaf7",
		"cn-northwest-1": "ami-0c023d9330c8e31a7",
		"eu-central-1":   "ami-02fc812f51a8d68b7",
		"eu-west-1":      "ami-058e9a0c98d35fe09",
		"eu-west-2":      "ami-0c841c182c761ff1b",
		"eu-south-1":     "ami-0e7360b35c785e3ff",
		"eu-west-3":      "ami-0e670dc7430624ced",
		"eu-north-1":     "ami-08b6aa83e0e2bd114",
		"me-south-1":     "ami-02438d916c9a15303",
		"sa-east-1":      "ami-02a2ba563262dd8c7",
	},
	"1.18": {
		"us-east-2":      "ami-0cdbd5e880e2d67df",
		"us-east-1":      "ami-0de537e93c519f366",
		"us-west-2":      "ami-02015388cf5b6a2e5",
		"us-west-1":      "ami-01c222d6652480da3",
		"af-south-1":     "ami-04c244b24fd041dca",
		"ap-east-1":      "ami-0bebc2701922e90bb",
		"ap-south-1":     "ami-0364ba4539e22b868",
		"ap-northeast-1": "ami-05c306ddcf8e239af",
		"ap-northeast-2": "ami-0e52578292f8ea07b",
		"ap-northeast-3": "ami-01b4c64ac11d11270",
		"ap-southeast-1": "ami-02837f897754aacf8",
		"ap-southeast-2": "ami-0f4069d877513fed3",
		"ca-central-1":   "ami-05e75ecfb5c9f2a73",
		"cn-north-1":     "ami-04c2ddc2c87ad14c6",
		"cn-northwest-1": "ami-088cc1ef35c151326",
		"eu-central-1":   "ami-0762397cb3a8ddfc4",
		"eu-west-1":      "ami-0675cb452a1793cea",
		"eu-west-2":      "ami-0a2a9b9c7536283c1",
		"eu-south-1":     "ami-06f7ef17456150db3",
		"eu-west-3":      "ami-0c3f884863ad4155e",
		"eu-north-1":     "ami-02f424f49b600a4ed",
		"me-south-1":     "ami-032dbe183ddc109f4",
		"sa-east-1":      "ami-048972b8c3f0b300d",
	},
	"1.17": {
		"us-east-2":      "ami-0d18632a6d937eac4",
		"us-east-1":      "ami-0c7e8d17364301c84",
		"us-west-2":      "ami-0491aee79d1e5805b",
		"us-west-1":      "ami-0919a29b6423beaad",
		"af-south-1":     "ami-090ef1d5bc7c4d0b1",
		"ap-east-1":      "ami-0fa7583d0bb0c3e63",
		"ap-south-1":     "ami-0a850ed842b706a63",
		"ap-northeast-1": "ami-06e863d4247f8b68d",
		"ap-northeast-2": "ami-0902b421f73756b36",
		"ap-northeast-3": "ami-0778ceabec835ecfa",
		"ap-southeast-1": "ami-07e8988702c4eac28",
		"ap-southeast-2": "ami-0213e285e24a1018c",
		"ca-central-1":   "ami-01ef04e6baaac4db3",
		"cn-north-1":     "ami-02a2b4b1eb9c7310d",
		"cn-northwest-1": "ami-079e5c15beff95376",
		"eu-central-1":   "ami-03d10d48a5fffeec7",
		"eu-west-1":      "ami-0496cf4d529058811",
		"eu-west-2":      "ami-0cec489756f45eda0",
		"eu-south-1":     "ami-0f37c4034397b519b",
		"eu-west-3":      "ami-059a69b312c673110",
		"eu-north-1":     "ami-097900540ac8427b9",
		"me-south-1":     "ami-08a2fe698592a4199",
		"sa-east-1":      "ami-032ce1c649ce6e1bf",
	},
	"1.16": {
		"us-east-2":      "ami-001a45719b7757a38",
		"us-east-1":      "ami-0aa255713ef4f3eac",
		"us-west-2":      "ami-0cd14786b7816816a",
		"us-west-1":      "ami-00cbb633d465d35bc",
		"af-south-1":     "ami-0199c85d302e4539c",
		"ap-east-1":      "ami-00cd38b6ac3021550",
		"ap-south-1":     "ami-0be82ef19d8a0c5da",
		"ap-northeast-1": "ami-051d6aee1004d3176",
		"ap-northeast-2": "ami-0ca5c7112731730af",
		"ap-northeast-3": "ami-03abcec389ba956d0",
		"ap-southeast-1": "ami-0e724a1405b6331d2",
		"ap-southeast-2": "ami-0025c36b72a5014ec",
		"ca-central-1":   "ami-061779ac60358d1ea",
		"cn-north-1":     "ami-053b04a1a7f3425a3",
		"cn-northwest-1": "ami-01877aa14e9fea5f2",
		"eu-central-1":   "ami-05777ea3f226eeb70",
		"eu-west-1":      "ami-08d8c404bc381220d",
		"eu-west-2":      "ami-044cb757f1cd8305c",
		"eu-south-1":     "ami-0ed54932726663755",
		"eu-west-3":      "ami-00785379b2aa94e08",
		"eu-north-1":     "ami-027bd84b47beb53a4",
		"me-south-1":     "ami-00a07d3539b9110f9",
		"sa-east-1":      "ami-0995cf0de9ca1fcd5",
	},
	"1.15": {
		"us-east-2":      "ami-040f577fe7a994dbf",
		"us-east-1":      "ami-02296bc1271d641bd",
		"us-west-2":      "ami-00032d029338cb595",
		"us-west-1":      "ami-004003baed7d48454",
		"af-south-1":     "ami-071962af6dfd9d060",
		"ap-east-1":      "ami-01fdd83a383cacc3e",
		"ap-south-1":     "ami-0529bbd8901c432a0",
		"ap-northeast-1": "ami-0bd8db55b76b389fc",
		"ap-northeast-2": "ami-08aa89bed5f2187cd",
		"ap-northeast-3": "ami-06d8a0a9a9df0191e",
		"ap-southeast-1": "ami-0f594a850603986a6",
		"ap-southeast-2": "ami-0b7529f8862c71beb",
		"ca-central-1":   "ami-091ffcab3e41b0be5",
		"cn-north-1":     "ami-0cc0079249ddd6a4c",
		"cn-northwest-1": "ami-0f1062576227a2811",
		"eu-central-1":   "ami-0abdbb3ca94efc0ca",
		"eu-west-1":      "ami-02d7b234725772a96",
		"eu-west-2":      "ami-02dfcf64bb9031273",
		"eu-south-1":     "ami-09b4acffff6a3bd14",
		"eu-west-3":      "ami-063cffebcaa304756",
		"eu-north-1":     "ami-0583cb4fd326a34dd",
		"me-south-1":     "ami-04e2515dbb216fdfc",
		"sa-east-1":      "ami-01b55d485d1a31178",
	},
}

type Driver struct {
	types.UnimplementedClusterSizeAccess
	types.UnimplementedVersionAccess

	driverCapabilities types.Capabilities

	request.Retryer
	metadata.ClientInfo

	Config   aws.Config
	Handlers request.Handlers
}

type state struct {
	ClusterName       string
	DisplayName       string
	ClientID          string
	ClientSecret      string
	SessionToken      string
	KeyPairName       string
	KubernetesVersion string

	MinimumASGSize int64
	MaximumASGSize int64
	DesiredASGSize int64
	NodeVolumeSize *int64
	EBSEncryption  bool

	UserData string

	InstanceType string
	Region       string

	VirtualNetwork              string
	Subnets                     []string
	SecurityGroups              []string
	ServiceRole                 string
	AMI                         string
	AssociateWorkerNodePublicIP *bool

	ClusterInfo types.ClusterInfo
}

func NewDriver() types.Driver {
	driver := &Driver{
		driverCapabilities: types.Capabilities{
			Capabilities: make(map[int64]bool),
		},
	}
	driver.driverCapabilities.AddCapability(types.GetVersionCapability)
	driver.driverCapabilities.AddCapability(types.SetVersionCapability)

	return driver
}

func (d *Driver) GetDriverCreateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}
	driverFlag.Options["display-name"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The displayed name of the cluster in the Rancher UI",
	}
	driverFlag.Options["access-key"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The AWS Client ID to use",
	}
	driverFlag.Options["secret-key"] = &types.Flag{
		Type:     types.StringType,
		Password: true,
		Usage:    "The AWS Client Secret associated with the Client ID",
	}
	driverFlag.Options["session-token"] = &types.Flag{
		Type:  types.StringType,
		Usage: "A session token to use with the client key and secret if applicable.",
	}
	driverFlag.Options["region"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The AWS Region to create the EKS cluster in",
		Default: &types.Default{
			DefaultString: "us-west-2",
		},
	}
	driverFlag.Options["instance-type"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The type of machine to use for worker nodes",
		Default: &types.Default{
			DefaultString: "t2.medium",
		},
	}
	driverFlag.Options["minimum-nodes"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The minimum number of worker nodes",
		Default: &types.Default{
			DefaultInt: 1,
		},
	}
	driverFlag.Options["maximum-nodes"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The maximum number of worker nodes",
		Default: &types.Default{
			DefaultInt: 3,
		},
	}
	driverFlag.Options["desired-nodes"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The desired number of worker nodes",
		Default: &types.Default{
			DefaultInt: 3,
		},
	}
	driverFlag.Options["node-volume-size"] = &types.Flag{
		Type:  types.IntPointerType,
		Usage: "The volume size for each node",
		Default: &types.Default{
			DefaultInt: 20,
		},
	}

	driverFlag.Options["virtual-network"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of the virtual network to use",
	}
	driverFlag.Options["subnets"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Comma-separated list of subnets in the virtual network to use",
		Default: &types.Default{
			DefaultStringSlice: &types.StringSlice{Value: []string{}}, // avoid nil value for init
		},
	}
	driverFlag.Options["service-role"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The service role to use to perform the cluster operations in AWS",
	}
	driverFlag.Options["security-groups"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Comma-separated list of security groups to use for the cluster",
	}
	driverFlag.Options["ami"] = &types.Flag{
		Type:  types.StringType,
		Usage: "A custom AMI ID to use for the worker nodes instead of the default",
	}
	driverFlag.Options["associate-worker-node-public-ip"] = &types.Flag{
		Type:  types.BoolPointerType,
		Usage: "A custom AMI ID to use for the worker nodes instead of the default",
		Default: &types.Default{
			DefaultBool: true,
		},
	}
	// Newlines are expected to always be passed as "\n"
	driverFlag.Options["user-data"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Pass user-data to the nodes to perform automated configuration tasks",
		Default: &types.Default{
			DefaultString: "#!/bin/bash\nset -o xtrace\n" +
				"/etc/eks/bootstrap.sh ${ClusterName} ${BootstrapArguments}\n" +
				"/opt/aws/bin/cfn-signal --exit-code $? " +
				"--stack  ${AWS::StackName} " +
				"--resource NodeGroup --region ${AWS::Region}",
		},
	}
	driverFlag.Options["keyPairName"] = &types.Flag{
		Type:  types.StringType,
		Usage: "Allow user to specify key name to use",
		Default: &types.Default{
			DefaultString: "",
		},
	}

	driverFlag.Options["kubernetes-version"] = &types.Flag{
		Type:    types.StringType,
		Usage:   "The kubernetes master version",
		Default: &types.Default{DefaultString: "1.13"},
	}
	driverFlag.Options["ebs-encryption"] = &types.Flag{
		Type:  types.BoolType,
		Usage: "Enables EBS encryption of worker nodes",
		Default: &types.Default{
			DefaultBool: false,
		},
	}

	return &driverFlag, nil
}

func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}

	driverFlag.Options["kubernetes-version"] = &types.Flag{
		Type:    types.StringType,
		Usage:   "The kubernetes version to update",
		Default: &types.Default{DefaultString: "1.13"},
	}
	driverFlag.Options["access-key"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The AWS Client ID to use",
	}
	driverFlag.Options["secret-key"] = &types.Flag{
		Type:     types.StringType,
		Password: true,
		Usage:    "The AWS Client Secret associated with the Client ID",
	}
	driverFlag.Options["session-token"] = &types.Flag{
		Type:  types.StringType,
		Usage: "A session token to use with the client key and secret if applicable.",
	}

	return &driverFlag, nil
}

func getStateFromOptions(driverOptions *types.DriverOptions) (state, error) {
	state := state{}
	state.ClusterName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "name").(string)
	state.DisplayName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "display-name", "displayName").(string)
	state.ClientID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-id", "accessKey").(string)
	state.ClientSecret = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-secret", "secretKey").(string)
	state.SessionToken = options.GetValueFromDriverOptions(driverOptions, types.StringType, "session-token", "sessionToken").(string)
	state.KubernetesVersion = options.GetValueFromDriverOptions(driverOptions, types.StringType, "kubernetes-version", "kubernetesVersion").(string)

	state.Region = options.GetValueFromDriverOptions(driverOptions, types.StringType, "region").(string)
	state.InstanceType = options.GetValueFromDriverOptions(driverOptions, types.StringType, "instance-type", "instanceType").(string)
	state.MinimumASGSize = options.GetValueFromDriverOptions(driverOptions, types.IntType, "minimum-nodes", "minimumNodes").(int64)
	state.MaximumASGSize = options.GetValueFromDriverOptions(driverOptions, types.IntType, "maximum-nodes", "maximumNodes").(int64)
	state.DesiredASGSize = options.GetValueFromDriverOptions(driverOptions, types.IntType, "desired-nodes", "desiredNodes").(int64)
	state.NodeVolumeSize, _ = options.GetValueFromDriverOptions(driverOptions, types.IntPointerType, "node-volume-size", "nodeVolumeSize").(*int64)
	state.VirtualNetwork = options.GetValueFromDriverOptions(driverOptions, types.StringType, "virtual-network", "virtualNetwork").(string)
	state.Subnets = options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "subnets").(*types.StringSlice).Value
	state.ServiceRole = options.GetValueFromDriverOptions(driverOptions, types.StringType, "service-role", "serviceRole").(string)
	state.SecurityGroups = options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "security-groups", "securityGroups").(*types.StringSlice).Value
	state.AMI = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ami").(string)
	state.AssociateWorkerNodePublicIP, _ = options.GetValueFromDriverOptions(driverOptions, types.BoolPointerType, "associate-worker-node-public-ip", "associateWorkerNodePublicIp").(*bool)
	state.KeyPairName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "keyPairName").(string)
	state.EBSEncryption = options.GetValueFromDriverOptions(driverOptions, types.BoolType, "ebsEncryption", "EBSEncryption").(bool)
	// UserData
	state.UserData = options.GetValueFromDriverOptions(driverOptions, types.StringType, "user-data", "userData").(string)

	return state, state.validate()
}

func (state *state) validate() error {
	if state.DisplayName == "" {
		return fmt.Errorf("display name is required")
	}

	if state.ClientID == "" {
		return fmt.Errorf("client id is required")
	}

	if state.ClientSecret == "" {
		return fmt.Errorf("client secret is required")
	}

	// If no k8s version is set then this is a legacy cluster and we can't choose the correct ami anyway, so skip those
	// validations
	if state.KubernetesVersion != "" {
		amiForRegion, ok := amiForRegionAndVersion[state.KubernetesVersion]
		if !ok && state.AMI == "" {
			return fmt.Errorf("default ami of region %s for kubernetes version %s is not set", state.Region, state.KubernetesVersion)
		}

		// If the custom AMI ID is set, then assume they are trying to spin up in a region we don't have knowledge of
		// and try to create anyway
		if amiForRegion[state.Region] == "" && state.AMI == "" {
			return fmt.Errorf("rancher does not support region %v, no entry for ami lookup", state.Region)
		}
	}

	if state.MinimumASGSize < 1 {
		return fmt.Errorf("minimum nodes must be greater than 0")
	}

	if state.MaximumASGSize < 1 {
		return fmt.Errorf("maximum nodes must be greater than 0")
	}

	if state.DesiredASGSize < 1 {
		return fmt.Errorf("desired nodes must be greater than 0")
	}

	if state.MaximumASGSize < state.MinimumASGSize {
		return fmt.Errorf("maximum nodes cannot be less than minimum nodes")
	}

	if state.DesiredASGSize < state.MinimumASGSize {
		return fmt.Errorf("desired nodes cannot be less than minimum nodes")
	}

	if state.DesiredASGSize > state.MaximumASGSize {
		return fmt.Errorf("desired nodes cannot be greater than maximum nodes")
	}

	if state.NodeVolumeSize != nil && *state.NodeVolumeSize < 1 {
		return fmt.Errorf("node volume size must be greater than 0")
	}

	networkEmpty := state.VirtualNetwork == ""
	subnetEmpty := len(state.Subnets) == 0
	securityGroupEmpty := len(state.SecurityGroups) == 0

	if !(networkEmpty == subnetEmpty && subnetEmpty == securityGroupEmpty) {
		return fmt.Errorf("virtual network, subnet, and security group must all be set together")
	}

	if state.AssociateWorkerNodePublicIP != nil &&
		!*state.AssociateWorkerNodePublicIP &&
		(state.VirtualNetwork == "" || len(state.Subnets) == 0) {
		return fmt.Errorf("if AssociateWorkerNodePublicIP is set to false a VPC and subnets must also be provided")
	}

	return nil
}

func alreadyExistsInCloudFormationError(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudformation.ErrCodeAlreadyExistsException:
			return true
		}
	}

	return false
}

func (d *Driver) createStack(svc *cloudformation.CloudFormation, name string, displayName string,
	templateBody string, capabilities []string, parameters []*cloudformation.Parameter) (*cloudformation.DescribeStacksOutput, error) {
	_, err := svc.CreateStack(&cloudformation.CreateStackInput{
		StackName:    aws.String(name),
		TemplateBody: aws.String(templateBody),
		Capabilities: aws.StringSlice(capabilities),
		Parameters:   parameters,
		Tags: []*cloudformation.Tag{
			{Key: aws.String("displayName"), Value: aws.String(displayName)},
		},
	})
	if err != nil && !alreadyExistsInCloudFormationError(err) {
		return nil, fmt.Errorf("error creating master: %v", err)
	}

	var stack *cloudformation.DescribeStacksOutput
	status := "CREATE_IN_PROGRESS"

	for status == "CREATE_IN_PROGRESS" {
		time.Sleep(time.Second * 5)
		stack, err = svc.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(name),
		})
		if err != nil {
			return nil, fmt.Errorf("error polling stack info: %v", err)
		}

		status = *stack.Stacks[0].StackStatus
	}

	if len(stack.Stacks) == 0 {
		return nil, fmt.Errorf("stack did not have output: %v", err)
	}

	if status != "CREATE_COMPLETE" {
		reason := "reason unknown"
		events, err := svc.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
			StackName: aws.String(name),
		})
		if err == nil {
			for _, event := range events.StackEvents {
				// guard against nil pointer dereference
				if event.ResourceStatus == nil || event.LogicalResourceId == nil || event.ResourceStatusReason == nil {
					continue
				}

				if *event.ResourceStatus == "CREATE_FAILED" {
					reason = *event.ResourceStatusReason
					break
				}

				if *event.ResourceStatus == "ROLLBACK_IN_PROGRESS" {
					reason = *event.ResourceStatusReason
					// do not break so that CREATE_FAILED takes priority
				}
			}
		}
		return nil, fmt.Errorf("stack failed to create: %v", reason)
	}

	return stack, nil
}

func toStringPointerSlice(strings []string) []*string {
	var stringPointers []*string

	for _, stringLiteral := range strings {
		stringPointers = append(stringPointers, aws.String(stringLiteral))
	}

	return stringPointers
}

func toStringLiteralSlice(strings []*string) []string {
	var stringLiterals []string

	for _, stringPointer := range strings {
		stringLiterals = append(stringLiterals, *stringPointer)
	}

	return stringLiterals
}

func (d *Driver) Create(ctx context.Context, options *types.DriverOptions, _ *types.ClusterInfo) (*types.ClusterInfo, error) {
	logrus.Infof("[amazonelasticcontainerservice] Starting create")

	state, err := getStateFromOptions(options)
	if err != nil {
		return nil, fmt.Errorf("error parsing state: %v", err)
	}

	info := &types.ClusterInfo{}
	if err := storeState(info, state); err != nil {
		return nil, fmt.Errorf("error storing eks state")
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(state.Region),
		Credentials: credentials.NewStaticCredentials(
			state.ClientID,
			state.ClientSecret,
			state.SessionToken,
		),
	})
	if err != nil {
		return info, fmt.Errorf("error getting new aws session: %v", err)
	}

	svc := cloudformation.New(sess)

	displayName := state.DisplayName

	var vpcid string
	var subnetIds []*string
	var securityGroups []*string
	if state.VirtualNetwork == "" {
		logrus.Infof("[amazonelasticcontainerservice] Bringing up vpc")

		stack, err := d.createStack(svc, getVPCStackName(state.DisplayName), displayName, vpcTemplate, []string{},
			[]*cloudformation.Parameter{})
		if err != nil {
			return info, fmt.Errorf("error creating stack with VPC template: %v", err)
		}

		securityGroupsString := getParameterValueFromOutput("SecurityGroups", stack.Stacks[0].Outputs)
		subnetIdsString := getParameterValueFromOutput("SubnetIds", stack.Stacks[0].Outputs)

		if securityGroupsString == "" || subnetIdsString == "" {
			return info, fmt.Errorf("no security groups or subnet ids were returned")
		}

		securityGroups = toStringPointerSlice(strings.Split(securityGroupsString, ","))
		subnetIds = toStringPointerSlice(strings.Split(subnetIdsString, ","))

		resources, err := svc.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
			StackName: aws.String(state.DisplayName + "-eks-vpc"),
		})
		if err != nil {
			return info, fmt.Errorf("error getting stack resources")
		}

		for _, resource := range resources.StackResources {
			if *resource.LogicalResourceId == "VPC" {
				vpcid = *resource.PhysicalResourceId
			}
		}
	} else {
		logrus.Infof("[amazonelasticcontainerservice] VPC info provided, skipping create")

		vpcid = state.VirtualNetwork
		subnetIds = toStringPointerSlice(state.Subnets)
		securityGroups = toStringPointerSlice(state.SecurityGroups)
	}

	var roleARN string
	if state.ServiceRole == "" {
		logrus.Infof("[amazonelasticcontainerservice] Creating service role")

		stack, err := d.createStack(svc, getServiceRoleName(state.DisplayName), displayName, serviceRoleTemplate,
			[]string{cloudformation.CapabilityCapabilityIam}, nil)
		if err != nil {
			return info, fmt.Errorf("error creating stack with service role template: %v", err)
		}

		roleARN = getParameterValueFromOutput("RoleArn", stack.Stacks[0].Outputs)
		if roleARN == "" {
			return info, fmt.Errorf("no RoleARN was returned")
		}
	} else {
		logrus.Infof("[amazonelasticcontainerservice] Retrieving existing service role")
		iamClient := iam.New(sess, aws.NewConfig().WithRegion(state.Region))
		role, err := iamClient.GetRole(&iam.GetRoleInput{
			RoleName: aws.String(state.ServiceRole),
		})
		if err != nil {
			return info, fmt.Errorf("error getting role: %v", err)
		}

		roleARN = *role.Role.Arn
	}

	logrus.Infof("[amazonelasticcontainerservice] Creating EKS cluster")

	eksService := eks.New(sess)
	_, err = eksService.CreateCluster(&eks.CreateClusterInput{
		Name:    aws.String(state.DisplayName),
		RoleArn: aws.String(roleARN),
		ResourcesVpcConfig: &eks.VpcConfigRequest{
			SecurityGroupIds: securityGroups,
			SubnetIds:        subnetIds,
		},
		Version: aws.String(state.KubernetesVersion),
	})
	if err != nil && !isClusterConflict(err) {
		return info, fmt.Errorf("error creating cluster: %v", err)
	}

	cluster, err := d.waitForClusterReady(eksService, state)
	if err != nil {
		return info, err
	}

	logrus.Infof("[amazonelasticcontainerservice] Cluster [%s] provisioned successfully", state.ClusterName)

	capem, err := base64.StdEncoding.DecodeString(*cluster.Cluster.CertificateAuthority.Data)
	if err != nil {
		return info, fmt.Errorf("error parsing CA data: %v", err)
	}
	// SSH Key pair creation
	ec2svc := ec2.New(sess)
	// make keyPairName visible outside of conditional scope
	keyPairName := state.KeyPairName

	if keyPairName == "" {
		keyPairName = getEC2KeyPairName(state.DisplayName)
		_, err = ec2svc.CreateKeyPair(&ec2.CreateKeyPairInput{
			KeyName: aws.String(keyPairName),
		})
	} else {
		_, err = ec2svc.CreateKeyPair(&ec2.CreateKeyPairInput{
			KeyName: aws.String(keyPairName),
		})
	}

	if err != nil && !isDuplicateKeyError(err) {
		return info, fmt.Errorf("error creating key pair %v", err)
	}

	logrus.Infof("[amazonelasticcontainerservice] Creating worker nodes")

	var amiID string
	if state.AMI != "" {
		amiID = state.AMI
	} else {
		// should be always accessible after validate()
		amiID = getAMIs(ctx, ec2svc, state)

	}

	var publicIP bool
	if state.AssociateWorkerNodePublicIP == nil {
		publicIP = true
	} else {
		publicIP = *state.AssociateWorkerNodePublicIP
	}
	// amend UserData values into template.
	// must use %q to safely pass the string
	workerNodesFinalTemplate := fmt.Sprintf(workerNodesTemplate, getEC2ServiceEndpoint(state.Region), state.UserData)

	var volumeSize int64
	if state.NodeVolumeSize == nil {
		volumeSize = 20
	} else {
		volumeSize = *state.NodeVolumeSize
	}

	stack, err := d.createStack(svc, getWorkNodeName(state.DisplayName), displayName, workerNodesFinalTemplate,
		[]string{cloudformation.CapabilityCapabilityIam},
		// these parameter values must exactly match those in the template file
		[]*cloudformation.Parameter{
			{ParameterKey: aws.String("ClusterName"), ParameterValue: aws.String(state.DisplayName)},
			{ParameterKey: aws.String("ClusterControlPlaneSecurityGroup"),
				ParameterValue: aws.String(strings.Join(toStringLiteralSlice(securityGroups), ","))},
			{ParameterKey: aws.String("NodeGroupName"),
				ParameterValue: aws.String(state.DisplayName + "-node-group")},
			{ParameterKey: aws.String("NodeAutoScalingGroupMinSize"), ParameterValue: aws.String(strconv.Itoa(
				int(state.MinimumASGSize)))},
			{ParameterKey: aws.String("NodeAutoScalingGroupMaxSize"), ParameterValue: aws.String(strconv.Itoa(
				int(state.MaximumASGSize)))},
			{ParameterKey: aws.String("NodeAutoScalingGroupDesiredCapacity"), ParameterValue: aws.String(strconv.Itoa(
				int(state.DesiredASGSize)))},
			{ParameterKey: aws.String("NodeVolumeSize"), ParameterValue: aws.String(strconv.Itoa(
				int(volumeSize)))},
			{ParameterKey: aws.String("NodeInstanceType"), ParameterValue: aws.String(state.InstanceType)},
			{ParameterKey: aws.String("NodeImageId"), ParameterValue: aws.String(amiID)},
			{ParameterKey: aws.String("KeyName"), ParameterValue: aws.String(keyPairName)},
			{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(vpcid)},
			{ParameterKey: aws.String("Subnets"),
				ParameterValue: aws.String(strings.Join(toStringLiteralSlice(subnetIds), ","))},
			{ParameterKey: aws.String("PublicIp"), ParameterValue: aws.String(strconv.FormatBool(publicIP))},
			{ParameterKey: aws.String("EBSEncryption"), ParameterValue: aws.String(strconv.FormatBool(state.EBSEncryption))},
		})
	if err != nil {
		return info, fmt.Errorf("error creating stack with worker nodes template: %v", err)
	}

	nodeInstanceRole := getParameterValueFromOutput("NodeInstanceRole", stack.Stacks[0].Outputs)
	if nodeInstanceRole == "" {
		return info, fmt.Errorf("no node instance role returned in output: %v", err)
	}

	err = d.createConfigMap(state, *cluster.Cluster.Endpoint, capem, nodeInstanceRole)
	if err != nil {
		return info, err
	}

	return info, nil
}

func isDuplicateKeyError(err error) bool {
	return strings.Contains(err.Error(), "already exists")
}

func isClusterConflict(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceInUseException
	}

	return false
}

func getEC2KeyPairName(name string) string {
	return name + "-ec2-key-pair"
}

func getServiceRoleName(name string) string {
	return name + "-eks-service-role"
}

func getVPCStackName(name string) string {
	return name + "-eks-vpc"
}

func getWorkNodeName(name string) string {
	return name + "-eks-worker-nodes"
}

func getEC2ServiceEndpoint(region string) string {
	if p, ok := endpoints.PartitionForRegion(endpoints.DefaultPartitions(), region); ok {
		return fmt.Sprintf("%s.%s", ec2.ServiceName, p.DNSSuffix())
	}
	return "ec2.amazonaws.com"
}

func (d *Driver) createConfigMap(state state, endpoint string, capem []byte, nodeInstanceRole string) error {
	clientset, err := createClientset(state.DisplayName, state, endpoint, capem)
	if err != nil {
		return fmt.Errorf("error creating clientset: %v", err)
	}

	data := []map[string]interface{}{
		{
			"rolearn":  nodeInstanceRole,
			"username": "system:node:{{EC2PrivateDNSName}}",
			"groups": []string{
				"system:bootstrappers",
				"system:nodes",
			},
		},
	}
	mapRoles, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshalling map roles: %v", err)
	}

	logrus.Infof("[amazonelasticcontainerservice] Applying ConfigMap")

	_, err = clientset.CoreV1().ConfigMaps("kube-system").Create(context.TODO(), &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws-auth",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"mapRoles": string(mapRoles),
		},
	}, metav1.CreateOptions{})
	if err != nil && !errors.IsConflict(err) {
		return fmt.Errorf("error creating config map: %v", err)
	}

	return nil
}

func createClientset(name string, state state, endpoint string, capem []byte) (*kubernetes.Clientset, error) {
	token, err := getEKSToken(name, state)
	if err != nil {
		return nil, fmt.Errorf("error generating token: %v", err)
	}

	config := &rest.Config{
		Host: endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: capem,
		},
		BearerToken: token,
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}

	return clientset, nil
}

const awsCredentialsDirectory = "./management-state/aws"
const awsCredentialsPath = awsCredentialsDirectory + "/credentials"
const awsSharedCredentialsFile = "AWS_SHARED_CREDENTIALS_FILE"

var awsCredentialsLocker = &sync.Mutex{}

func getEKSToken(name string, state state) (string, error) {
	generator, err := heptio.NewGenerator()
	if err != nil {
		return "", fmt.Errorf("error creating generator: %v", err)
	}

	defer awsCredentialsLocker.Unlock()
	awsCredentialsLocker.Lock()
	os.Setenv(awsSharedCredentialsFile, awsCredentialsPath)

	defer func() {
		os.Remove(awsCredentialsPath)
		os.Remove(awsCredentialsDirectory)
		os.Unsetenv(awsSharedCredentialsFile)
	}()
	err = os.MkdirAll(awsCredentialsDirectory, 0744)
	if err != nil {
		return "", fmt.Errorf("error creating credentials directory: %v", err)
	}

	var credentialsContent string
	if state.SessionToken == "" {
		credentialsContent = fmt.Sprintf(
			`[default]
aws_access_key_id=%v
aws_secret_access_key=%v
region=%v`,
			state.ClientID,
			state.ClientSecret,
			state.Region)
	} else {
		credentialsContent = fmt.Sprintf(
			`[default]
aws_access_key_id=%v
aws_secret_access_key=%v
aws_session_token=%v
region=%v`,
			state.ClientID,
			state.ClientSecret,
			state.SessionToken,
			state.Region)
	}

	err = ioutil.WriteFile(awsCredentialsPath, []byte(credentialsContent), 0644)
	if err != nil {
		return "", fmt.Errorf("error writing credentials file: %v", err)
	}

	return generator.Get(name)
}

func (d *Driver) waitForClusterReady(svc *eks.EKS, state state) (*eks.DescribeClusterOutput, error) {
	var cluster *eks.DescribeClusterOutput
	var err error

	status := ""
	for status != eks.ClusterStatusActive {
		time.Sleep(30 * time.Second)

		logrus.Infof("[amazonelasticcontainerservice] Waiting for cluster [%s] to finish provisioning", state.ClusterName)

		cluster, err = svc.DescribeCluster(&eks.DescribeClusterInput{
			Name: aws.String(state.DisplayName),
		})
		if err != nil {
			return nil, fmt.Errorf("error polling cluster state: %v", err)
		}

		if cluster.Cluster == nil {
			return nil, fmt.Errorf("no cluster data was returned")
		}

		if cluster.Cluster.Status == nil {
			return nil, fmt.Errorf("no cluster status was returned")
		}

		status = *cluster.Cluster.Status

		if status == eks.ClusterStatusFailed {
			return nil, fmt.Errorf("creation failed for cluster named %q with ARN %q",
				aws.StringValue(cluster.Cluster.Name),
				aws.StringValue(cluster.Cluster.Arn))
		}
	}

	return cluster, nil
}

func storeState(info *types.ClusterInfo, state state) error {
	data, err := json.Marshal(state)

	if err != nil {
		return err
	}

	if info.Metadata == nil {
		info.Metadata = map[string]string{}
	}

	info.Metadata["state"] = string(data)

	return nil
}

func getState(info *types.ClusterInfo) (state, error) {
	state := state{}

	err := json.Unmarshal([]byte(info.Metadata["state"]), &state)
	if err != nil {
		logrus.Errorf("Error encountered while marshalling state: %v", err)
	}

	return state, err
}

func getParameterValueFromOutput(key string, outputs []*cloudformation.Output) string {
	for _, output := range outputs {
		if *output.OutputKey == key {
			return *output.OutputValue
		}
	}

	return ""
}

func (d *Driver) Update(ctx context.Context, info *types.ClusterInfo, options *types.DriverOptions) (*types.ClusterInfo, error) {
	logrus.Infof("[amazonelasticcontainerservice] Starting update")
	oldstate := &state{}
	state, err := getState(info)
	if err != nil {
		return nil, err
	}
	*oldstate = state

	newState, err := getStateFromOptions(options)
	if err != nil {
		return nil, err
	}

	var sendUpdate bool
	if newState.KubernetesVersion != "" && newState.KubernetesVersion != state.KubernetesVersion {
		sendUpdate = true
		state.KubernetesVersion = newState.KubernetesVersion
	}

	if newState.ClientID != "" && newState.ClientID != state.ClientID {
		state.ClientID = newState.ClientID
	}

	if newState.ClientSecret != "" && newState.ClientSecret != state.ClientSecret {
		state.ClientSecret = newState.ClientSecret
	}

	if !sendUpdate {
		logrus.Infof("[amazonelasticcontainerservice] Update complete for cluster [%s]", state.ClusterName)
		return info, storeState(info, state)
	}

	if err := d.updateClusterAndWait(ctx, state); err != nil {
		logrus.Errorf("error updating cluster: %v", err)
		return info, err
	}

	logrus.Infof("[amazonelasticcontainerservice] Update complete for cluster [%s]", state.ClusterName)
	return info, storeState(info, state)
}

func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	logrus.Infof("[amazonelasticcontainerservice] Starting post-check")

	clientset, err := getClientset(info)
	if err != nil {
		return nil, err
	}

	logrus.Infof("[amazonelasticcontainerservice] Generating service account token")

	info.ServiceAccountToken, err = util.GenerateServiceAccountToken(clientset)
	if err != nil {
		return nil, fmt.Errorf("error generating service account token: %v", err)
	}

	return info, nil
}

func getClientset(info *types.ClusterInfo) (*kubernetes.Clientset, error) {
	state, err := getState(info)
	if err != nil {
		return nil, err
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(state.Region),
		Credentials: credentials.NewStaticCredentials(
			state.ClientID,
			state.ClientSecret,
			state.SessionToken,
		),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating new session: %v", err)
	}

	svc := eks.New(sess)
	cluster, err := svc.DescribeCluster(&eks.DescribeClusterInput{
		Name: aws.String(state.DisplayName),
	})
	if err != nil {
		if notFound(err) {
			cluster, err = svc.DescribeCluster(&eks.DescribeClusterInput{
				Name: aws.String(state.ClusterName),
			})
		}

		if err != nil {
			return nil, fmt.Errorf("error getting cluster: %v", err)
		}
	}

	capem, err := base64.StdEncoding.DecodeString(*cluster.Cluster.CertificateAuthority.Data)
	if err != nil {
		return nil, fmt.Errorf("error parsing CA data: %v", err)
	}

	info.Endpoint = *cluster.Cluster.Endpoint
	info.Version = *cluster.Cluster.Version
	info.RootCaCertificate = *cluster.Cluster.CertificateAuthority.Data

	clientset, err := createClientset(state.DisplayName, state, *cluster.Cluster.Endpoint, capem)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}

	_, err = clientset.ServerVersion()
	if err != nil {
		if errors.IsUnauthorized(err) {
			clientset, err = createClientset(state.ClusterName, state, *cluster.Cluster.Endpoint, capem)
			if err != nil {
				return nil, err
			}

			_, err = clientset.ServerVersion()
		}

		if err != nil {
			return nil, err
		}
	}

	return clientset, nil
}

func (d *Driver) Remove(ctx context.Context, info *types.ClusterInfo) error {
	logrus.Infof("[amazonelasticcontainerservice] Starting delete cluster")

	state, err := getState(info)
	if err != nil {
		return fmt.Errorf("error getting state: %v", err)
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(state.Region),
		Credentials: credentials.NewStaticCredentials(
			state.ClientID,
			state.ClientSecret,
			state.SessionToken,
		),
	})
	if err != nil {
		return fmt.Errorf("error getting new aws session: %v", err)
	}

	eksSvc := eks.New(sess)
	_, err = eksSvc.DeleteCluster(&eks.DeleteClusterInput{
		Name: aws.String(state.DisplayName),
	})
	if err != nil {
		if notFound(err) {
			_, err = eksSvc.DeleteCluster(&eks.DeleteClusterInput{
				Name: aws.String(state.ClusterName),
			})
		}

		if err != nil && !notFound(err) {
			return fmt.Errorf("error deleting cluster: %v", err)
		}
	}

	svc := cloudformation.New(sess)

	err = deleteStack(svc, getServiceRoleName(state.DisplayName), getServiceRoleName(state.ClusterName))
	if err != nil {
		return fmt.Errorf("error deleting service role stack: %v", err)
	}

	err = deleteStack(svc, getVPCStackName(state.DisplayName), getVPCStackName(state.ClusterName))
	if err != nil {
		return fmt.Errorf("error deleting vpc stack: %v", err)
	}

	err = deleteStack(svc, getWorkNodeName(state.DisplayName), getWorkNodeName(state.ClusterName))
	if err != nil {
		return fmt.Errorf("error deleting worker node stack: %v", err)
	}

	ec2svc := ec2.New(sess)

	name := state.DisplayName
	_, err = ec2svc.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
		KeyNames: []*string{aws.String(getEC2KeyPairName(name))},
	})
	if doesNotExist(err) {
		name = state.ClusterName
	}

	_, err = ec2svc.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: aws.String(getEC2KeyPairName(name)),
	})
	if err != nil {
		return fmt.Errorf("error deleting key pair: %v", err)
	}

	return err
}

func deleteStack(svc *cloudformation.CloudFormation, newStyleName, oldStyleName string) error {
	name := newStyleName
	_, err := svc.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(name),
	})
	if doesNotExist(err) {
		name = oldStyleName
	}

	_, err = svc.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(name),
	})
	if err != nil {
		return fmt.Errorf("error deleting stack: %v", err)
	}

	return nil
}

func notFound(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceNotFoundException
	}

	return false
}

func invalidParam(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeInvalidParameterException
	}

	return false
}

func doesNotExist(err error) bool {
	// There is no better way of doing this because AWS API does not distinguish between a attempt to delete a stack
	// (or key pair) that does not exist, and, for example, a malformed delete request, so we have to parse the error
	// message
	if err != nil {
		return strings.Contains(err.Error(), "does not exist")
	}

	return false
}

func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &d.driverCapabilities, nil
}

func (d *Driver) ETCDSave(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) ETCDRestore(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) (*types.ClusterInfo, error) {
	return nil, fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) ETCDRemoveSnapshot(ctx context.Context, clusterInfo *types.ClusterInfo, opts *types.DriverOptions, snapshotName string) error {
	return fmt.Errorf("ETCD backup operations are not implemented")
}

func (d *Driver) GetK8SCapabilities(ctx context.Context, _ *types.DriverOptions) (*types.K8SCapabilities, error) {
	return &types.K8SCapabilities{
		L4LoadBalancer: &types.LoadBalancerCapabilities{
			Enabled:              true,
			Provider:             "ELB",
			ProtocolsSupported:   []string{"TCP"},
			HealthCheckSupported: true,
		},
	}, nil
}

func (d *Driver) GetVersion(ctx context.Context, info *types.ClusterInfo) (*types.KubernetesVersion, error) {
	cluster, err := d.getClusterStats(ctx, info)
	if err != nil {
		return nil, err
	}

	version := &types.KubernetesVersion{Version: *cluster.Version}

	return version, nil
}

func (d *Driver) getClusterStats(ctx context.Context, info *types.ClusterInfo) (*eks.Cluster, error) {
	state, err := getState(info)
	if err != nil {
		return nil, err
	}
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(state.Region),
		Credentials: credentials.NewStaticCredentials(
			state.ClientID,
			state.ClientSecret,
			state.SessionToken,
		),
	})
	if err != nil {
		return nil, err
	}

	svc := eks.New(sess)
	cluster, err := svc.DescribeClusterWithContext(ctx, &eks.DescribeClusterInput{
		Name: aws.String(state.DisplayName),
	})
	if err != nil {
		return nil, err
	}

	return cluster.Cluster, nil
}

func (d *Driver) SetVersion(ctx context.Context, info *types.ClusterInfo, version *types.KubernetesVersion) error {
	logrus.Info("[amazonelasticcontainerservice] updating kubernetes version")
	state, err := getState(info)
	if err != nil {
		return err
	}

	state.KubernetesVersion = version.Version
	if err := d.updateClusterAndWait(ctx, state); err != nil {
		return err
	}

	logrus.Info("[amazonelasticcontainerservice] kubernetes version update success")
	return nil
}

func (d *Driver) updateClusterAndWait(ctx context.Context, state state) error {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(state.Region),
		Credentials: credentials.NewStaticCredentials(
			state.ClientID,
			state.ClientSecret,
			state.SessionToken,
		),
	})
	if err != nil {
		return err
	}

	svc := eks.New(sess)
	input := &eks.UpdateClusterVersionInput{
		Name: aws.String(state.DisplayName),
	}
	if state.KubernetesVersion != "" {
		input.Version = aws.String(state.KubernetesVersion)
	}

	output, err := svc.UpdateClusterVersionWithContext(ctx, input)
	if err != nil {
		if notFound(err) {
			input.Name = aws.String(state.ClusterName)
			output, err = svc.UpdateClusterVersionWithContext(ctx, input)
		}
		if invalidParam(err) {
			describeInput := &eks.DescribeClusterInput{
				Name: input.Name,
			}
			eksState, describeErr := svc.DescribeCluster(describeInput)
			if describeErr != nil {
				// show original error encountered during update
				return err
			}
			if eksState == nil || eksState.Cluster == nil {
				return err
			}
			if eksState.Cluster.Version != nil && *eksState.Cluster.Version == state.KubernetesVersion {
				// versions match, no update needed
				return nil
			}
		}
		if err != nil {
			return err
		}
	}

	return d.waitForClusterUpdateReady(ctx, svc, state, *output.Update.Id)
}

func (d *Driver) waitForClusterUpdateReady(ctx context.Context, svc *eks.EKS, state state, updateID string) error {
	logrus.Infof("[amazonelasticcontainerservice] waiting for update id[%s] state", updateID)
	var update *eks.DescribeUpdateOutput
	var err error

	status := ""
	for status != "Successful" {
		time.Sleep(30 * time.Second)

		logrus.Infof("[amazonelasticcontainerservice] Waiting for cluster [%s] update to finish updating", state.ClusterName)

		update, err = svc.DescribeUpdateWithContext(ctx, &eks.DescribeUpdateInput{
			Name:     aws.String(state.DisplayName),
			UpdateId: aws.String(updateID),
		})
		if err != nil {
			if notFound(err) {
				update, err = svc.DescribeUpdateWithContext(ctx, &eks.DescribeUpdateInput{
					Name:     aws.String(state.ClusterName),
					UpdateId: aws.String(updateID),
				})
			}

			if err != nil {
				return fmt.Errorf("error polling cluster update state: %v", err)
			}
		}

		if update.Update == nil {
			return fmt.Errorf("no cluster update data was returned")
		}

		if update.Update.Status == nil {
			return fmt.Errorf("no cluster update status aws returned")
		}

		status = *update.Update.Status
	}

	return nil
}

func getAMIs(ctx context.Context, ec2svc *ec2.EC2, state state) string {
	if rtn := getLocalAMI(state); rtn != "" {
		return rtn
	}
	version := state.KubernetesVersion
	output, err := ec2svc.DescribeImagesWithContext(ctx, &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("is-public"),
				Values: aws.StringSlice([]string{"true"}),
			},
			{
				Name:   aws.String("state"),
				Values: aws.StringSlice([]string{"available"}),
			},
			{
				Name:   aws.String("image-type"),
				Values: aws.StringSlice([]string{"machine"}),
			},
			{
				Name:   aws.String("name"),
				Values: aws.StringSlice([]string{fmt.Sprintf("%s%s*", amiNamePrefix, version)}),
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Warn("getting AMIs from aws error")
		return ""
	}
	prefix := fmt.Sprintf("%s%s", amiNamePrefix, version)
	rtnImage := ""
	for _, image := range output.Images {
		if *image.State != "available" ||
			*image.ImageType != "machine" ||
			!*image.Public ||
			image.Name == nil ||
			!strings.HasPrefix(*image.Name, prefix) {
			continue
		}
		if *image.ImageId > rtnImage {
			rtnImage = *image.ImageId
		}
	}
	if rtnImage == "" {
		logrus.Warnf("no AMI id was returned")
		return ""
	}
	return rtnImage
}

func getLocalAMI(state state) string {
	amiForRegion, ok := amiForRegionAndVersion[state.KubernetesVersion]
	if !ok {
		return ""
	}
	return amiForRegion[state.Region]
}

func (d *Driver) RemoveLegacyServiceAccount(ctx context.Context, info *types.ClusterInfo) error {
	clientset, err := getClientset(info)
	if err != nil {
		return nil
	}

	return util.DeleteLegacyServiceAccountAndRoleBinding(clientset)
}
