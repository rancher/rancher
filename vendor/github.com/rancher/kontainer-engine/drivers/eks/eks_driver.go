package eks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/client/metadata"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	heptio "github.com/heptio/authenticator/pkg/token"
	"github.com/rancher/kontainer-engine/drivers/options"
	"github.com/rancher/kontainer-engine/drivers/util"
	"github.com/rancher/kontainer-engine/types"
	"github.com/sirupsen/logrus"
	"github.com/smartystreets/go-aws-auth"
	"gopkg.in/yaml.v2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var amiForRegion = map[string]string{
	"us-west-2": "ami-0a54c984b9f908c81",
	"us-east-1": "ami-0440e4f6b9713faf6",
	"eu-west-1": "ami-0c7a4976cb6fafd3a",
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
	ClusterName  string
	DisplayName  string
	ClientID     string
	ClientSecret string

	MinimumASGSize int64
	MaximumASGSize int64

	InstanceType string
	Region       string

	VirtualNetwork string
	Subnets        []string
	SecurityGroups []string
	ServiceRole    string
	AMI            string

	ClusterInfo types.ClusterInfo
}

type eksCluster struct {
	Cluster clusterObj `json:"cluster"`
}

type clusterObj struct {
	MasterEndpoint       *string              `json:"endpoint"`
	ClusterName          *string              `json:"name"`
	Status               *string              `json:"status"`
	CreatedAt            *float64             `json:"createdAt"`
	DesiredMasterVersion *string              `json:"desiredMasterVersion"`
	VPCID                *string              `json:"vpcId"`
	CurrentMasterVersion *string              `json:"currentMasterVersion"`
	RoleARN              *string              `json:"roleArn"`
	CertificateAuthority certificateAuthority `json:"certificateAuthority"`
	ResourcesVPCConfig   vpcConfig            `json:"resourcesVpcConfig"`
}

type vpcConfig struct {
	SecurityGroups []string `json:"securityGroupIds"`
	Subnets        []string `json:"subnetIds"`
}

type certificateAuthority struct {
	Data *string `json:"data"`
}

func NewDriver() types.Driver {
	driver := &Driver{
		driverCapabilities: types.Capabilities{
			Capabilities: make(map[int64]bool),
		},
	}

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
	driverFlag.Options["client-id"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The AWS Client ID to use",
	}
	driverFlag.Options["client-secret"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The AWS Client Secret associated with the Client ID",
	}
	driverFlag.Options["region"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The AWS Region to create the EKS cluster in",
		Value: "us-west-2",
	}
	driverFlag.Options["instance-type"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The type of machine to use for worker nodes",
		Value: "t2.medium",
	}
	driverFlag.Options["minimum-nodes"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The minimum number of worker nodes",
		Value: "1",
	}
	driverFlag.Options["maximum-nodes"] = &types.Flag{
		Type:  types.IntType,
		Usage: "The maximum number of worker nodes",
		Value: "3",
	}

	driverFlag.Options["virtual-network"] = &types.Flag{
		Type:  types.StringType,
		Usage: "The name of hte virtual network to use",
	}
	driverFlag.Options["subnets"] = &types.Flag{
		Type:  types.StringSliceType,
		Usage: "Comma-separated list of subnets in the virtual network to use",
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

	return &driverFlag, nil
}

func (d *Driver) GetDriverUpdateOptions(ctx context.Context) (*types.DriverFlags, error) {
	driverFlag := types.DriverFlags{
		Options: make(map[string]*types.Flag),
	}

	return &driverFlag, nil
}

func getStateFromOptions(driverOptions *types.DriverOptions) (state, error) {
	state := state{}
	state.ClusterName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "name").(string)
	state.DisplayName = options.GetValueFromDriverOptions(driverOptions, types.StringType, "display-name", "displayName").(string)
	state.ClientID = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-id", "accessKey").(string)
	state.ClientSecret = options.GetValueFromDriverOptions(driverOptions, types.StringType, "client-secret", "secretKey").(string)

	state.Region = options.GetValueFromDriverOptions(driverOptions, types.StringType, "region").(string)
	state.InstanceType = options.GetValueFromDriverOptions(driverOptions, types.StringType, "instance-type", "instanceType").(string)
	state.MinimumASGSize = options.GetValueFromDriverOptions(driverOptions, types.IntType, "minimum-nodes", "minimumNodes").(int64)
	state.MaximumASGSize = options.GetValueFromDriverOptions(driverOptions, types.IntType, "maximum-nodes", "maximumNodes").(int64)
	state.VirtualNetwork = options.GetValueFromDriverOptions(driverOptions, types.StringType, "virtual-network", "virtualNetwork").(string)
	state.Subnets = options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "subnets").(*types.StringSlice).Value
	state.ServiceRole = options.GetValueFromDriverOptions(driverOptions, types.StringType, "service-role", "serviceRole").(string)
	state.SecurityGroups = options.GetValueFromDriverOptions(driverOptions, types.StringSliceType, "security-groups", "securityGroups").(*types.StringSlice).Value
	state.AMI = options.GetValueFromDriverOptions(driverOptions, types.StringType, "ami").(string)

	return state, state.validate()
}

func (state *state) validate() error {
	if state.ClusterName == "" {
		return fmt.Errorf("cluster name is required")
	}

	if state.ClientID == "" {
		return fmt.Errorf("client id is required")
	}

	if state.ClientSecret == "" {
		return fmt.Errorf("client secret is required")
	}

	// If the custom AMI ID is set, then assume they are trying to spin up in a region we don't have knowledge of
	// and try to create anyway
	if amiForRegion[state.Region] == "" && state.AMI == "" {
		return fmt.Errorf("rancher does not support region %v, no entry for ami lookup", state.Region)
	}

	if state.MinimumASGSize < 1 {
		return fmt.Errorf("minimum nodes must be greater than 0")
	}

	if state.MaximumASGSize < 1 {
		return fmt.Errorf("maximum nodes must be greater than 0")
	}

	if state.MaximumASGSize < state.MinimumASGSize {
		return fmt.Errorf("maximum nodes cannot be less than minimum nodes")
	}

	networkEmpty := state.VirtualNetwork == ""
	subnetEmpty := len(state.Subnets) == 0
	securityGroupEmpty := len(state.SecurityGroups) == 0

	if !(networkEmpty == subnetEmpty && subnetEmpty == securityGroupEmpty) {
		return fmt.Errorf("virtual network, subnet, and security group must all be set together")
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
	templateURL string, parameters []*cloudformation.Parameter) (*cloudformation.DescribeStacksOutput, error) {
	_, err := svc.CreateStack(&cloudformation.CreateStackInput{
		StackName:   aws.String(name),
		TemplateURL: aws.String(templateURL),
		Capabilities: aws.StringSlice([]string{
			cloudformation.CapabilityCapabilityIam,
		}),
		Parameters: parameters,
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
			}
		}
		return nil, fmt.Errorf("stack failed to create: %v", reason)
	}

	return stack, nil
}

func (d *Driver) awsHTTPRequest(state state, url string, method string, data []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url,
		bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("error creating http request: %v", err)
	}

	client := http.DefaultClient

	req.Header.Set("Content-Type", "application/json")

	awsauth.Sign4(req, awsauth.Credentials{
		AccessKeyID:     state.ClientID,
		SecretAccessKey: state.ClientSecret,
	})

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error creating cluster: %v", err)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body) // always read the body so the caller can get the error message from it

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return body, fmt.Errorf("%v", resp.StatusCode)
	}

	if err != nil {
		return nil, fmt.Errorf("error reading body: %v", err)
	}

	return body, nil
}

func (d *Driver) Create(ctx context.Context, options *types.DriverOptions, _ *types.ClusterInfo) (*types.ClusterInfo, error) {
	logrus.Infof("Starting create")

	state, err := getStateFromOptions(options)
	if err != nil {
		return nil, fmt.Errorf("error parsing state: %v", err)
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(state.Region),
		Credentials: credentials.NewStaticCredentials(
			state.ClientID,
			state.ClientSecret,
			"",
		),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting new aws session: %v", err)
	}

	svc := cloudformation.New(sess)

	displayName := state.DisplayName
	if displayName == "" {
		displayName = state.ClusterName
	}

	var vpcid string
	var subnetIds []string
	var securityGroups []string
	if state.VirtualNetwork == "" {
		logrus.Infof("Bringing up vpc")

		stack, err := d.createStack(svc, getVPCStackName(state), displayName,
			"https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-08-30/amazon-eks-vpc-sample.yaml",
			[]*cloudformation.Parameter{})
		if err != nil {
			return nil, fmt.Errorf("error creating stack: %v", err)
		}

		securityGroupsString := getParameterValueFromOutput("SecurityGroups", stack.Stacks[0].Outputs)
		subnetIdsString := getParameterValueFromOutput("SubnetIds", stack.Stacks[0].Outputs)

		if securityGroupsString == "" || subnetIdsString == "" {
			return nil, fmt.Errorf("no security groups or subnet ids were returned")
		}

		securityGroups = strings.Split(securityGroupsString, ",")
		subnetIds = strings.Split(subnetIdsString, ",")

		resources, err := svc.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
			StackName: aws.String(state.ClusterName + "-eks-vpc"),
		})
		if err != nil {
			return nil, fmt.Errorf("error getting stack resoures")
		}

		for _, resource := range resources.StackResources {
			if *resource.LogicalResourceId == "VPC" {
				vpcid = *resource.PhysicalResourceId
			}
		}
	} else {
		logrus.Infof("VPC info provided, skipping create")

		vpcid = state.VirtualNetwork
		subnetIds = state.Subnets
		securityGroups = state.SecurityGroups
	}

	var roleARN string
	if state.ServiceRole == "" {
		logrus.Infof("Creating service role")

		stack, err := d.createStack(svc, getServiceRoleName(state), displayName,
			"https://amazon-eks.s3-us-west-2.amazonaws.com/1.10.3/2018-06-05/amazon-eks-service-role.yaml", nil)
		if err != nil {
			return nil, fmt.Errorf("error creating stack: %v", err)
		}

		roleARN = getParameterValueFromOutput("RoleArn", stack.Stacks[0].Outputs)
		if roleARN == "" {
			return nil, fmt.Errorf("no RoleARN was returned")
		}
	} else {
		logrus.Infof("Retrieving existing service role")
		iamClient := iam.New(sess, aws.NewConfig().WithRegion(state.Region))
		role, err := iamClient.GetRole(&iam.GetRoleInput{
			RoleName: aws.String(state.ServiceRole),
		})
		if err != nil {
			return nil, fmt.Errorf("error getting role: %v", err)
		}

		roleARN = *role.Role.Arn
	}

	data, err := json.Marshal(&clusterObj{
		ClusterName: aws.String(state.ClusterName),
		RoleARN:     aws.String(roleARN),
		ResourcesVPCConfig: vpcConfig{
			SecurityGroups: securityGroups,
			Subnets:        subnetIds,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error marshalling eks cluster: %v", err)
	}

	logrus.Infof("Creating EKS cluster")

	body, err := d.awsHTTPRequest(state, fmt.Sprintf("https://eks.%v.amazonaws.com/clusters", state.Region), "POST", data)
	if err != nil && !isClusterConflict(err) {
		return nil, fmt.Errorf("error creating cluster: %v %v", err, string(body))
	}

	cluster, err := d.waitForClusterReady(state)
	if err != nil {
		return nil, err
	}

	logrus.Infof("Cluster provisioned successfully")

	capem, err := base64.StdEncoding.DecodeString(*cluster.Cluster.CertificateAuthority.Data)
	if err != nil {
		return nil, fmt.Errorf("error parsing CA data: %v", err)
	}

	ec2svc := ec2.New(sess)
	keyPairName := getEC2KeyPairName(state)
	_, err = ec2svc.CreateKeyPair(&ec2.CreateKeyPairInput{
		KeyName: aws.String(keyPairName),
	})
	if err != nil && !isDuplicateKeyError(err) {
		return nil, fmt.Errorf("error creating key pair %v", err)
	}

	logrus.Infof("Creating worker nodes")

	var amiID string
	if state.AMI != "" {
		amiID = state.AMI
	} else {
		amiID = amiForRegion[state.Region]
	}

	stack, err := d.createStack(svc, getWorkNodeName(state), displayName,
		"https://amazon-eks.s3-us-west-2.amazonaws.com/cloudformation/2018-08-30/amazon-eks-nodegroup.yaml",
		[]*cloudformation.Parameter{
			{ParameterKey: aws.String("ClusterName"), ParameterValue: aws.String(state.ClusterName)},
			{ParameterKey: aws.String("ClusterControlPlaneSecurityGroup"),
				ParameterValue: aws.String(strings.Join(securityGroups, ","))},
			{ParameterKey: aws.String("NodeGroupName"),
				ParameterValue: aws.String(state.ClusterName + "-node-group")},
			{ParameterKey: aws.String("NodeAutoScalingGroupMinSize"), ParameterValue: aws.String(strconv.Itoa(
				int(state.MinimumASGSize)))},
			{ParameterKey: aws.String("NodeAutoScalingGroupMaxSize"), ParameterValue: aws.String(strconv.Itoa(
				int(state.MaximumASGSize)))},
			{ParameterKey: aws.String("NodeInstanceType"), ParameterValue: aws.String(state.InstanceType)},
			{ParameterKey: aws.String("NodeImageId"), ParameterValue: aws.String(amiID)},
			{ParameterKey: aws.String("KeyName"), ParameterValue: aws.String(keyPairName)}, // TODO let the user specify this
			{ParameterKey: aws.String("VpcId"), ParameterValue: aws.String(vpcid)},
			{ParameterKey: aws.String("Subnets"),
				ParameterValue: aws.String(strings.Join(subnetIds, ","))},
		})
	if err != nil {
		return nil, fmt.Errorf("error creating stack: %v", err)
	}

	nodeInstanceRole := getParameterValueFromOutput("NodeInstanceRole", stack.Stacks[0].Outputs)
	if nodeInstanceRole == "" {
		return nil, fmt.Errorf("no node instance role returned in output: %v", err)
	}

	err = d.createConfigMap(state, *cluster.Cluster.MasterEndpoint, capem, nodeInstanceRole)
	if err != nil {
		return nil, err
	}

	info := &types.ClusterInfo{}
	storeState(info, state)
	return info, nil
}

func isDuplicateKeyError(err error) bool {
	return strings.Contains(err.Error(), "already exists")
}

func isClusterConflict(err error) bool {
	return err.Error() == "409"
}

func getEC2KeyPairName(state state) string {
	return state.ClusterName + "-ec2-key-pair"
}

func getServiceRoleName(state state) string {
	return state.ClusterName + "-eks-service-role"
}

func getVPCStackName(state state) string {
	return state.ClusterName + "-eks-vpc"
}

func (d *Driver) createConfigMap(state state, endpoint string, capem []byte, nodeInstanceRole string) error {
	clientset, err := getClientset(state, endpoint, capem)
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

	logrus.Infof("Applying ConfigMap")

	_, err = clientset.CoreV1().ConfigMaps("kube-system").Create(&v1.ConfigMap{
		TypeMeta: v12.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v12.ObjectMeta{
			Name:      "aws-auth",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"mapRoles": string(mapRoles),
		},
	})
	if err != nil && !errors.IsConflict(err) {
		return fmt.Errorf("error creating config map: %v", err)
	}

	return nil
}

func getClientset(state state, endpoint string, capem []byte) (*kubernetes.Clientset, error) {
	token, err := getEKSToken(state)
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

func getEKSToken(state state) (string, error) {
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

	err = ioutil.WriteFile(awsCredentialsPath, []byte(fmt.Sprintf(
		`[default]
aws_access_key_id=%v
aws_secret_access_key=%v`,
		state.ClientID,
		state.ClientSecret)), 0644)
	if err != nil {
		return "", fmt.Errorf("error writing credentials file: %v", err)
	}

	return generator.Get(state.ClusterName)
}

func (d *Driver) waitForClusterReady(state state) (*eksCluster, error) {
	cluster := &eksCluster{}

	status := ""
	for status != "ACTIVE" {
		time.Sleep(30 * time.Second)

		logrus.Infof("Waiting for cluster to finish provisioning")

		resp, err := d.awsHTTPRequest(state, fmt.Sprintf("https://eks.%v.amazonaws.com/clusters/%v",
			state.Region, state.ClusterName),
			"GET", nil)
		if err != nil {
			return nil, fmt.Errorf("error getting cluster: %v", err)
		}

		err = json.Unmarshal(resp, cluster)
		if err != nil {
			return nil, fmt.Errorf("error parsing cluster: %v", err)
		}

		if cluster.Cluster.Status == nil {
			return nil, fmt.Errorf("no cluster status was returned")
		}

		status = *cluster.Cluster.Status
	}

	return cluster, nil
}

func getWorkNodeName(state state) string {
	return state.ClusterName + "-eks-worker-nodes"
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
	logrus.Infof("Starting update")

	// nothing can be updated so just return

	logrus.Infof("Update complete")
	return info, nil
}

func (d *Driver) PostCheck(ctx context.Context, info *types.ClusterInfo) (*types.ClusterInfo, error) {
	logrus.Infof("Starting post-check")

	state, err := getState(info)
	if err != nil {
		return nil, err
	}

	resp, err := d.awsHTTPRequest(state, fmt.Sprintf("https://eks.%v.amazonaws.com/clusters/%v", state.Region,
		state.ClusterName), "GET", nil)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster: %v", err)
	}

	cluster := &eksCluster{}

	err = json.Unmarshal(resp, &cluster)
	if err != nil {
		return nil, fmt.Errorf("error parsing cluster: %v", err)
	}

	capem, err := base64.StdEncoding.DecodeString(*cluster.Cluster.CertificateAuthority.Data)
	if err != nil {
		return nil, fmt.Errorf("error parsing CA data: %v", err)
	}

	info.Endpoint = *cluster.Cluster.MasterEndpoint
	info.RootCaCertificate = *cluster.Cluster.CertificateAuthority.Data

	clientset, err := getClientset(state, *cluster.Cluster.MasterEndpoint, capem)
	if err != nil {
		return nil, fmt.Errorf("error creating clientset: %v", err)
	}

	logrus.Infof("Generating service account token")

	info.ServiceAccountToken, err = util.GenerateServiceAccountToken(clientset)
	if err != nil {
		return nil, fmt.Errorf("error generating service account token: %v", err)
	}

	return info, nil
}

func (d *Driver) Remove(ctx context.Context, info *types.ClusterInfo) error {
	logrus.Infof("Starting delete cluster")

	state, err := getState(info)
	if err != nil {
		return fmt.Errorf("error getting state: %v", err)
	}

	_, err = d.awsHTTPRequest(state, fmt.Sprintf("https://eks.%v.amazonaws.com/clusters/%v", state.Region,
		state.ClusterName), "DELETE", nil)
	if err != nil && !noClusterFound(err) {
		return fmt.Errorf("error deleting cluster: %v", err)
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(state.Region),
		Credentials: credentials.NewStaticCredentials(
			state.ClientID,
			state.ClientSecret,
			"",
		),
	})
	if err != nil {
		return fmt.Errorf("error getting new aws session: %v", err)
	}

	svc := cloudformation.New(sess)

	for _, stackName := range []string{getServiceRoleName(state), getVPCStackName(state), getWorkNodeName(state)} {
		_, err = svc.DeleteStack(&cloudformation.DeleteStackInput{
			StackName: aws.String(stackName),
		})
		if err != nil {
			return fmt.Errorf("error deleting stack: %v", err)
		}
	}

	ec2svc := ec2.New(sess)

	_, err = ec2svc.DeleteKeyPair(&ec2.DeleteKeyPairInput{
		KeyName: aws.String(getEC2KeyPairName(state)),
	})
	if err != nil {
		return fmt.Errorf("error deleting key pair: %v", err)
	}

	return nil
}

func noClusterFound(err error) bool {
	return strings.Contains(err.Error(), "No cluster found for name")
}

func (d *Driver) GetCapabilities(ctx context.Context) (*types.Capabilities, error) {
	return &d.driverCapabilities, nil
}
