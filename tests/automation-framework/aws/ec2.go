package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/rancher/tests/automation-framework/config"
)

type EC2Client struct {
	svc       *ec2.EC2
	Nodes     []*EC2Node
	awsConfig *config.AWSConfig
}

func NewEC2Client(awsConfig *config.AWSConfig) (*EC2Client, error) {
	credential := credentials.NewStaticCredentials(awsConfig.GetAWSAccessKeyID(), awsConfig.GetAWSSecretAccessKey(), "")
	sess, err := session.NewSession(&aws.Config{
		Credentials: credential,
		Region:      aws.String(awsConfig.GetAWSRegion())},
	)
	if err != nil {
		return nil, err
	}

	// Create EC2 service client
	svc := ec2.New(sess)
	return &EC2Client{
		svc:       svc,
		awsConfig: awsConfig,
	}, nil
}

func (e *EC2Client) CreateNodes(nodeNameBase string, publicIp bool, numOfInstancs int64) (func() error, error) {
	sshName := getSSHKeyName(e.awsConfig.GetAWSSSHKeyName())

	runInstancesInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(e.awsConfig.GetAWSSSHKeyName()),
		InstanceType: aws.String(e.awsConfig.GetAWSSSHKeyName()),
		MinCount:     aws.Int64(numOfInstancs),
		MaxCount:     aws.Int64(numOfInstancs),
		KeyName:      aws.String(sshName),
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize: aws.Int64(int64(e.awsConfig.GetAWSVolumeSize())),
				},
			},
		},
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: aws.String(e.awsConfig.GetAWSIAMProfile()),
		},
		Placement: &ec2.Placement{
			AvailabilityZone: aws.String(e.awsConfig.GetAWSRegionAZ()),
		},
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int64(0),
				AssociatePublicIpAddress: aws.Bool(publicIp),
				Groups:                   aws.StringSlice([]string{e.awsConfig.GetAWSSecurityGroup()}),
			},
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(nodeNameBase),
					},
					{
						Key:   aws.String("CICD"),
						Value: aws.String(e.awsConfig.GetAWSCICDInstanceTag()),
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

	insanceCleanup := func() error {
		err := deleteNodes(listOfInstanceIds, e.svc)
		return err
	}

	//wait until instance is running
	err = e.svc.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return insanceCleanup, err
	}

	//wait until instance status is ok
	err = e.svc.WaitUntilInstanceStatusOk(&ec2.DescribeInstanceStatusInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return insanceCleanup, err
	}

	// describe instance to get attributes=
	describe, err := e.svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return insanceCleanup, err
	}

	readyInstances := describe.Reservations[0].Instances

	sshKey, err := getSSHKey(e.awsConfig.GetAWSSSHKeyName())
	if err != nil {
		return insanceCleanup, err
	}

	var ec2Nodes []*EC2Node

	for _, readyInstance := range readyInstances {
		ec2Node := &EC2Node{
			NodeName:         *readyInstance.State.Name,
			NodeID:           *readyInstance.InstanceId,
			PublicIPAdress:   *readyInstance.PublicIpAddress,
			PrivateIPAddress: *readyInstance.PrivateIpAddress,
			SSHUser:          e.awsConfig.GetAWSUser(),
			SSHKey:           sshKey,
		}
		ec2Nodes = append(ec2Nodes, ec2Node)
	}
	e.Nodes = append(e.Nodes, ec2Nodes...)

	return insanceCleanup, nil
}

func deleteNodes(instanceIDs []*string, svc *ec2.EC2) error {
	_, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	})

	if err != nil {
		return err
	}

	return nil
}
