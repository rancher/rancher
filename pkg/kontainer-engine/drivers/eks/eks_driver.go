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
	"1.17": {
		"eu-north-1":     "ami-0e2a534d4199a9d33",
		"ap-south-1":     "ami-07f9a71ddb518de63",
		"eu-west-3":      "ami-0daa783a28b127c5e",
		"eu-west-2":      "ami-006e34c0fd206303c",
		"eu-west-1":      "ami-0af93ebcaa763f50b",
		"ap-northeast-2": "ami-046851e3e2f884ddb",
		"ap-northeast-1": "ami-08c36bc5fca5ac740",
		"sa-east-1":      "ami-029dfe70c00fba190",
		"ca-central-1":   "ami-0ef7d671940c2700e",
		"ap-southeast-1": "ami-03f2cc02e61de355f",
		"ap-southeast-2": "ami-034808d124324b1ab",
		"eu-central-1":   "ami-047e3ad49b70ed809",
		"us-east-1":      "ami-02a815c648e3ac746",
		"us-east-2":      "ami-0154136a3710f138e",
		"us-west-1":      "ami-0719c98155584b5d3",
		"us-west-2":      "ami-05c646a801901b785",
		"ap-east-1":      "ami-004c6d9e8cfe1a936",
		"cn-north-1":     "ami-0bd0f836c99725df4",
		"cn-northwest-1": "ami-09be27ee84f66b04f",
	},
	"1.16": {
		"eu-north-1":     "ami-0dcf932e4747e34b4",
		"ap-south-1":     "ami-039f05d4b231576ff",
		"eu-west-3":      "ami-0002b3f8441a3baa9",
		"eu-west-2":      "ami-04e1815ffe8b842f1",
		"eu-west-1":      "ami-016ce7a1ae8ecbc95",
		"ap-northeast-2": "ami-0c47d4fe48c39208b",
		"ap-northeast-1": "ami-00f674be5ba7f332e",
		"sa-east-1":      "ami-01327ee038a367f65",
		"ca-central-1":   "ami-0f2f572f876bc3a79",
		"ap-southeast-1": "ami-03094f3697947d7fb",
		"ap-southeast-2": "ami-0e9142c4f75390efa",
		"eu-central-1":   "ami-0b2edbf190fe05b92",
		"us-east-1":      "ami-03e6aee2536bf785f",
		"us-east-2":      "ami-06d3900b00136f924",
		"us-west-1":      "ami-01411ee43e8e470d8",
		"us-west-2":      "ami-02b73604aad3f32f1",
		"cn-north-1":     "ami-06ab9635ff6edebf6",
		"cn-northwest-1": "ami-095af05476e5b9cf8",
		"ap-east-1":      "ami-0d2c7412f58efa398",
	},
	"1.15": {
		"eu-north-1":     "ami-0250975944c3540a3",
		"ap-south-1":     "ami-06bb2e8f92371c753",
		"eu-west-3":      "ami-04c0bcdae408ea2fa",
		"eu-west-2":      "ami-047f33d5928fd8770",
		"eu-west-1":      "ami-0e7c6091d33e3debd",
		"ap-northeast-2": "ami-071c9373382b6a8ed",
		"ap-northeast-1": "ami-0ecf35150cfc9a353",
		"sa-east-1":      "ami-09235b76d9357ca23",
		"ca-central-1":   "ami-0d5ebbe84f08f42fb",
		"ap-southeast-1": "ami-0504b3f2ecc102dda",
		"ap-southeast-2": "ami-021d8395ff6ca79f7",
		"eu-central-1":   "ami-0e1553c48d514ef82",
		"us-east-1":      "ami-0ebc74bed6ea6c804",
		"us-east-2":      "ami-055fe3896d448e22d",
		"us-west-1":      "ami-0710c7ce7e3daf595",
		"us-west-2":      "ami-06162493555d2273d",
		"cn-north-1":     "ami-05b179a26f95a62db",
		"cn-northwest-1": "ami-039044adc28bd70e9",
		"ap-east-1":      "ami-02e7df97f15c8338b",
	},
	"1.14": {
		"eu-north-1":     "ami-011c9f6c87115ef00",
		"ap-south-1":     "ami-0d77c6bd6dc359ad5",
		"eu-west-3":      "ami-08a0d97d0a60a4456",
		"eu-west-2":      "ami-0c35e2d22b8934100",
		"eu-west-1":      "ami-07c336c3dda9111bb",
		"ap-northeast-2": "ami-0a5d761d87031ed1c",
		"ap-northeast-1": "ami-09c377c608a717353",
		"sa-east-1":      "ami-0dd924ad6c84376a9",
		"ca-central-1":   "ami-0cc036f6e236114da",
		"ap-southeast-1": "ami-0aa0f841c32906d83",
		"ap-southeast-2": "ami-0fb4d1493eec4e078",
		"eu-central-1":   "ami-07d042e2ff229d19f",
		"us-east-1":      "ami-0531cd86ccb972e2a",
		"us-east-2":      "ami-037b2a7fa8e6ebeba",
		"us-west-1":      "ami-0cc31102b72fc3261",
		"us-west-2":      "ami-077e2a60357cb396f",
		"cn-north-1":     "ami-0e4cb8067782b693a",
		"cn-northwest-1": "ami-047594a1f70bab3e2",
		"ap-east-1":      "ami-037be1cd6655e254e",
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
			DefaultStringSlice: &types.StringSlice{Value: []string{}}, //avoid nil value for init
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
		//should be always accessible after validate()
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
