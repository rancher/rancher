package aws

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/rancher/tests/go-validation/environmentvariables"
)

var AWSInstanceType string = environmentvariables.Getenv("AWS_INSTANCE_TYPE", "t3a.medium")
var AWSRegion = environmentvariables.Getenv("AWS_REGION", "us-east-2")
var AWSRegionAZ = environmentvariables.Getenv("AWS_REGION_AZ", "")
var AWSAMI = environmentvariables.Getenv("AWS_AMI", "ami-0d5d9d301c853a04a")
var AWSSecurityGroup = environmentvariables.Getenv("AWS_SECURITY_GROUPS", "sg-0e753fd5550206e55")
var awsAccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
var AWSSecretAccessKey = environmentvariables.Getenv("AWS_SECRET_ACCESS_KEY", "jenkins-rke-validation.pem")
var AWSSSHKeyName = os.Getenv("AWS_SSH_KEY_NAME")
var AWSCICDInstanceTag = environmentvariables.Getenv("AWS_CICD_INSTANCE_TAG", "rancher-validation")
var AWSIAMProfile = environmentvariables.Getenv("AWS_IAM_PROFILE", "")
var AWSUser = environmentvariables.Getenv("AWS_USER", "ubuntu")
var AWSVolumeSize = environmentvariables.ConvertStringToInt(environmentvariables.Getenv("AWS_VOLUME_SIZE", "50"))

type EC2Client struct {
	svc *ec2.EC2
}

func NewEC2Client() (*EC2Client, error) {
	credential := credentials.NewStaticCredentials(awsAccessKeyID, AWSSecretAccessKey, "")
	sess, err := session.NewSession(&aws.Config{
		Credentials: credential,
		Region:      aws.String(AWSRegion)},
	)
	if err != nil {
		return nil, err
	}

	// Create EC2 service client
	svc := ec2.New(sess)
	return &EC2Client{
		svc: svc,
	}, nil
}

func (e *EC2Client) CreateNodes(nodeNameBase string, publicIp bool, numOfInstancs int64) ([]*EC2Node, error) {
	sshName := getSSHKeyName(AWSSSHKeyName)

	runInstancesInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(AWSAMI),
		InstanceType: aws.String(AWSInstanceType),
		MinCount:     aws.Int64(numOfInstancs),
		MaxCount:     aws.Int64(numOfInstancs),
		KeyName:      aws.String(sshName),
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			&ec2.BlockDeviceMapping{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize: aws.Int64(int64(AWSVolumeSize)),
				},
			},
		},
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: aws.String(AWSIAMProfile),
		},
		Placement: &ec2.Placement{
			AvailabilityZone: aws.String(AWSRegionAZ),
		},
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			&ec2.InstanceNetworkInterfaceSpecification{
				DeviceIndex:              aws.Int64(0),
				AssociatePublicIpAddress: aws.Bool(publicIp),
				Groups:                   aws.StringSlice([]string{AWSSecurityGroup}),
			},
		},
		TagSpecifications: []*ec2.TagSpecification{
			&ec2.TagSpecification{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					&ec2.Tag{
						Key:   aws.String("Name"),
						Value: aws.String(nodeNameBase),
					},
					&ec2.Tag{
						Key:   aws.String("CICD"),
						Value: aws.String(AWSCICDInstanceTag),
					},
				},
			},
		},
	}

	reservation, err := e.svc.RunInstances(runInstancesInput)
	if err != nil {
		return nil, err
	}

	var listOfInstanceIds []*string

	for _, instance := range reservation.Instances {
		listOfInstanceIds = append(listOfInstanceIds, instance.InstanceId)
	}

	//wait until instance is running
	err = e.svc.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return nil, err
	}

	//wait until instance status is ok
	err = e.svc.WaitUntilInstanceStatusOk(&ec2.DescribeInstanceStatusInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return nil, err
	}

	// describe instance to get attributes=
	describe, err := e.svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return nil, err
	}

	readyInstances := describe.Reservations[0].Instances

	sshKey, err := getSSHKey(AWSSSHKeyName)
	if err != nil {
		return nil, err
	}

	var ec2Nodes []*EC2Node

	for _, readyInstance := range readyInstances {
		ec2Node := &EC2Node{
			NodeName:         *readyInstance.State.Name,
			NodeID:           *readyInstance.InstanceId,
			PublicIPAdress:   *readyInstance.PublicIpAddress,
			PrivateIPAddress: *readyInstance.PrivateIpAddress,
			SSHUser:          AWSUser,
			SSHKey:           sshKey,
		}
		ec2Nodes = append(ec2Nodes, ec2Node)
	}

	return ec2Nodes, nil
}

func (e *EC2Client) DeleteNodes(nodes []*EC2Node) (*ec2.TerminateInstancesOutput, error) {
	var instanceIDs []*string

	for _, node := range nodes {
		instanceIDs = append(instanceIDs, aws.String(node.NodeID))
	}

	return e.svc.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	})
}
