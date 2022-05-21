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
	"1.22": {
		"us-east-2":      "ami-05c1b5a6e6a6fa16e",
		"us-east-1":      "ami-0281494f7c4eb37d1",
		"us-west-2":      "ami-0c8f79982ab160bad",
		"us-west-1":      "ami-0642b600309077ef5",
		"af-south-1":     "ami-03d9c66d3577a670d",
		"ap-east-1":      "ami-0794b6da10eeb6fe8",
		"ap-south-1":     "ami-0e070d28b9ba80b5f",
		"ap-northeast-1": "ami-0df4e9519930dec44",
		"ap-northeast-2": "ami-006da1df7b3f0314c",
		"ap-northeast-3": "ami-09b7c559361080c12",
		"ap-southeast-1": "ami-091a4816b24a28609",
		"ap-southeast-2": "ami-05ed20b703d2f11a1",
		"ap-southeast-3": "ami-0b6514b2d21ad4b86",
		"ca-central-1":   "ami-0adb2ff3a246d92f0",
		"cn-north-1":     "ami-048d9a2c626ffcf03",
		"cn-northwest-1": "ami-09c3d3a3b5f9c7225",
		"eu-central-1":   "ami-00a3108f45c6dbb6c",
		"eu-west-1":      "ami-0123eaf3fc3256084",
		"eu-west-2":      "ami-0d184db0f7d78e409",
		"eu-south-1":     "ami-0a90bc77ae459e737",
		"eu-west-3":      "ami-0572799da7d7711e5",
		"eu-north-1":     "ami-03e9fa5d35116a5df",
		"me-south-1":     "ami-0db42d7c87caa5a01",
		"sa-east-1":      "ami-0a459177cd5b03219",
	},
	"1.21": {
		"us-east-2":      "ami-0b1eb76fbce602b88",
		"us-east-1":      "ami-05911b9b4df1172c7",
		"us-west-2":      "ami-0927a66ff40101d76",
		"us-west-1":      "ami-0b231db44d895f441",
		"af-south-1":     "ami-0c816ea216fa6ee91",
		"ap-east-1":      "ami-0f2e8a7df065de79d",
		"ap-south-1":     "ami-083fc9a94c76fcf99",
		"ap-northeast-1": "ami-02c6d763272789974",
		"ap-northeast-2": "ami-0a935a0be13d15a62",
		"ap-northeast-3": "ami-04b6b2ffed6e017cd",
		"ap-southeast-1": "ami-06c6b04b283f6a360",
		"ap-southeast-2": "ami-04844e9ed76402c4c",
		"ap-southeast-3": "ami-0bc53493c9819ddd4",
		"ca-central-1":   "ami-08fbcecd888e28020",
		"cn-north-1":     "ami-0a14e0a7620dfeba1",
		"cn-northwest-1": "ami-0133e8290205b7280",
		"eu-central-1":   "ami-0cdaae2396feeac04",
		"eu-west-1":      "ami-003f91f482a604b6d",
		"eu-west-2":      "ami-03bfc3ec1fbb98b64",
		"eu-south-1":     "ami-030365a090aaeea5f",
		"eu-west-3":      "ami-055d6d194079fd39c",
		"eu-north-1":     "ami-0eb7a1eaa3452a92e",
		"me-south-1":     "ami-0e8d0f4ea83543ec4",
		"sa-east-1":      "ami-044c743d56df85e52",
	},
	"1.20": {
		"us-east-2":      "ami-0098e60c6f91e0198",
		"us-east-1":      "ami-01a09362cdb8f50b3",
		"us-west-2":      "ami-018dff183a28ac510",
		"us-west-1":      "ami-05a3f9078be6d6b29",
		"af-south-1":     "ami-02b509d5c16d495ab",
		"ap-east-1":      "ami-01dfb0851a0b6079a",
		"ap-south-1":     "ami-0941443c8917f415a",
		"ap-northeast-1": "ami-050c02862539f4984",
		"ap-northeast-2": "ami-0e34fc9c7492756ee",
		"ap-northeast-3": "ami-07acd78236d1e11dc",
		"ap-southeast-1": "ami-0bf1fa15b81394b79",
		"ap-southeast-2": "ami-041460ef88863e87a",
		"ap-southeast-3": "ami-0ba189db2ca65051c",
		"ca-central-1":   "ami-05d993a0162ed26a2",
		"cn-north-1":     "ami-090ae5a23cc3258b0",
		"cn-northwest-1": "ami-0451054a087e850e4",
		"eu-central-1":   "ami-0b273062ba87d6a40",
		"eu-west-1":      "ami-0e9af5e112a678f08",
		"eu-west-2":      "ami-013244fec0ed69e0c",
		"eu-south-1":     "ami-0aa02cb747c8555f8",
		"eu-west-3":      "ami-0878e400a1097c4d4",
		"eu-north-1":     "ami-0a89934258901e12e",
		"me-south-1":     "ami-0d6f9aa783431f2fb",
		"sa-east-1":      "ami-04cd0cef082181df6",
	},
	"1.19": {
		"us-east-2":      "ami-0dce41d8956099d1c",
		"us-east-1":      "ami-0ad9600b3719f8a53",
		"us-west-2":      "ami-0123cdc1d3e4fac7a",
		"us-west-1":      "ami-08d4cdd9fa56e4cc2",
		"af-south-1":     "ami-07588447be81a4ea7",
		"ap-east-1":      "ami-05a2763a066421d82",
		"ap-south-1":     "ami-01de2b54dee5461ba",
		"ap-northeast-1": "ami-08eb69ce386c9e5d4",
		"ap-northeast-2": "ami-0b164ef1518193966",
		"ap-northeast-3": "ami-0f86193ab75516a4e",
		"ap-southeast-1": "ami-0c814bff86ffae438",
		"ap-southeast-2": "ami-074d8646b8184300b",
		"ap-southeast-3": "ami-029f3d45ab365c296",
		"ca-central-1":   "ami-07c6ae17e840017d8",
		"cn-north-1":     "ami-0e0dd9802118f43db",
		"cn-northwest-1": "ami-01857a5d6877f1c87",
		"eu-central-1":   "ami-0c03b017874e1d7e4",
		"eu-west-1":      "ami-07e16eb3fc2c12328",
		"eu-west-2":      "ami-06178c718a7d9b164",
		"eu-south-1":     "ami-0009a866467edf6d1",
		"eu-west-3":      "ami-02fe296802edc3bf9",
		"eu-north-1":     "ami-02cc2d248cab6d882",
		"me-south-1":     "ami-0a4140bed808f4ec1",
		"sa-east-1":      "ami-0af82db946859c80f",
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
