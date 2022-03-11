package ec2

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
)

const (
	nodeBaseName               = "rancherautomation"
	LocalWindowsPEMKeyName     = "windows-ec2-key.pem"
	WindowsPemKeyName          = "windows-pem-automation"
	sshPath                    = ".ssh"
	defaultWindowsVolumeSize   = int(100)
	defaultWindowsInstanceType = "m5a.xlarge"
)

// CreateNodes CreatedNodes creates `numOfInstances` number of ec2 instances
func CreateNodes(client *rancher.Client, numOfInstances int) ([]*nodes.Node, error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return nil, err
	}

	hasWindows := false
	if ec2Client.Config.InstanceTypeWindows != "" || ec2Client.Config.AWSWindowsAMI != "" || ec2Client.Config.VolumeSizeWindows != 0 {
		hasWindows = true
	}

	var listOfInstanceIds []*string

	// Create Linux Nodes
	runInstancesInput, err := createNodesCommon(client, numOfInstances, false)
	if err != nil {
		return nil, err
	}

	reservation, err := ec2Client.SVC.RunInstances(runInstancesInput)
	if err != nil {
		return nil, err
	}
	for _, instance := range reservation.Instances {
		listOfInstanceIds = append(listOfInstanceIds, instance.InstanceId)
	}

	// Create Windows Nodes
	var windowsInstanceID string
	var windowsUserData = `<powershell>\nAdd-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0\nStart-Service ssh-agent; Start-Service sshd\nSet-Service -Name sshd -StartupType 'Automatic'\nSet-Service docker -StartUpType Disabled -Status Stopped\nStop-Process dockerd\n</powershell>"`
	if hasWindows {
		runInstancesInput.UserData = aws.String(windowsUserData)
		runInstancesInput, err = createNodesCommon(client, 1, hasWindows)
		if err != nil {
			return nil, err
		}

		reservation, err = ec2Client.SVC.RunInstances(runInstancesInput)
		if err != nil {
			return nil, err
		}
		for _, instance := range reservation.Instances {
			windowsInstanceID = aws.StringValue(instance.InstanceId)
			listOfInstanceIds = append(listOfInstanceIds, instance.InstanceId)
		}
	}

	//wait until instance is running
	err = ec2Client.SVC.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return nil, err
	}

	//wait until instance status is ok
	err = ec2Client.SVC.WaitUntilInstanceStatusOk(&ec2.DescribeInstanceStatusInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return nil, err
	}

	// wait for Windows password to be available
	var nodePassword *string

	if hasWindows {
		pwInput := &ec2.GetPasswordDataInput{
			InstanceId: aws.String(windowsInstanceID),
		}
		err = ec2Client.SVC.WaitUntilPasswordDataAvailable(pwInput)
		if err != nil {
			return nil, err
		}
		pwData, err := ec2Client.SVC.GetPasswordData(pwInput)
		if err != nil {
			return nil, err
		}
		nodePassword = pwData.PasswordData
	}

	// describe instance to get attributes
	describe, err := ec2Client.SVC.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return nil, err
	}

	readyInstances := describe.Reservations[0].Instances

	sshKey, err := nodes.GetSSHKey(ec2Client.Config.AWSSSHKeyName, hasWindows)
	if err != nil {
		return nil, err
	}

	var ec2Nodes []*nodes.Node
	for _, readyInstance := range readyInstances {
		ec2Node := &nodes.Node{
			NodeID:          aws.StringValue(readyInstance.InstanceId),
			PublicIPAddress: aws.StringValue(readyInstance.PublicIpAddress),
			SSHUser:         ec2Client.Config.AWSUser,
			SSHKey:          sshKey,
			NodePassword:    aws.StringValue(nodePassword),
		}
		ec2Nodes = append(ec2Nodes, ec2Node)
	}
	client.Session.RegisterCleanupFunc(func() error {
		return DeleteNodes(client, ec2Nodes)
	})

	return ec2Nodes, nil
}

// DeleteNodes terminates ec2 instances that have been created.
func DeleteNodes(client *rancher.Client, nodes []*nodes.Node) error {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return err
	}
	var instanceIDs []*string

	for _, node := range nodes {
		instanceIDs = append(instanceIDs, aws.String(node.NodeID))
	}

	_, err = ec2Client.SVC.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: instanceIDs,
	})
	return err
}

func getSSHKeyName(sshKeyName string) string {
	stringSlice := strings.Split(sshKeyName, ".")
	return stringSlice[0]
}

// create PEM for Windows Instances
func generatePEM() (string, error) {
	input := ec2.CreateKeyPairInput{}
	keyName := provisioning.AppendRandomString(WindowsPemKeyName)
	pemKey := input.SetKeyName(keyName)
	output := ec2.CreateKeyPairOutput{KeyName: pemKey.KeyName}
	sensitivePEM := output.KeyMaterial
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(filepath.Join(user.HomeDir, sshPath), 0744)
	if err != nil {
		return "", err
	}
	localPEM := filepath.Join(user.HomeDir, sshPath, LocalWindowsPEMKeyName)
	err = os.WriteFile(localPEM, []byte(convert.ToString(sensitivePEM)), 0400)
	if err != nil {
		os.Remove(localPEM)
		return "", err
	}

	return keyName, nil
}

func createNodesCommon(client *rancher.Client, numOfInstances int, hasWindows bool) (*ec2.RunInstancesInput, error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return nil, err
	}
	keyName := getSSHKeyName(ec2Client.Config.AWSSSHKeyName)
	imageID := ec2Client.Config.AWSLinuxAMI
	instanceType := ec2Client.Config.InstanceTypeLinux
	volumeSize := ec2Client.Config.VolumeSizeLinux

	if hasWindows {
		keyName, err = generatePEM()
		if err != nil {
			return nil, err
		}
		filterValues := []string{"platform=windows,architecture=x86_64,is-public=true"}
		f := &ec2.Filter{
			Name:   aws.String(""),
			Values: aws.StringSlice(filterValues),
		}
		input := ec2.DescribeImagesInput{}
		filters := ec2.DescribeImagesInput{}.Filters
		filters = append(filters, f)

		imageFilters, err := ec2Client.SVC.DescribeImages(&input)
		if err != nil {
			return nil, err
		}
		// todo: fix
		imageID = imageFilters.GoString()
		instanceType = defaultWindowsInstanceType
		volumeSize = defaultWindowsVolumeSize
	}

	return &ec2.RunInstancesInput{
		ImageId:      aws.String(imageID),
		InstanceType: aws.String(instanceType),
		MinCount:     aws.Int64(int64(numOfInstances)),
		MaxCount:     aws.Int64(int64(numOfInstances)),
		KeyName:      aws.String(keyName),
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2.EbsBlockDevice{
					VolumeType:          aws.String("gp3"),
					DeleteOnTermination: aws.Bool(true),
					VolumeSize:          aws.Int64(int64(volumeSize)),
				},
			},
		},
		IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
			Name: aws.String(ec2Client.Config.AWSIAMProfile),
		},
		Placement: &ec2.Placement{
			AvailabilityZone: aws.String(ec2Client.Config.AWSRegionAZ),
		},
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int64(0),
				AssociatePublicIpAddress: aws.Bool(true),
				Groups:                   aws.StringSlice(ec2Client.Config.AWSSecurityGroups),
			},
		},
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("instance"),
				Tags: []*ec2.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(nodeBaseName),
					},
					{
						Key:   aws.String("CICD"),
						Value: aws.String(ec2Client.Config.AWSCICDInstanceTag),
					},
				},
			},
		},
	}, nil
}
