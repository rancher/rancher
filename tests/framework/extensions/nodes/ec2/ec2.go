package ec2

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
)

const (
	nodeBaseName               = "rancherautomation"
	defaultWindowsVolumeSize   = int(100)
	defaultWindowsInstanceType = "m5a.xlarge"
)

// CreateNodes creates `numOfInstances` number of ec2 instances
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
	runInstancesInput, err := createNodesCommon(client, numOfInstances, false, "")
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

	sshKey, err := nodes.GetSSHKey(ec2Client.Config.AWSSSHKeyName)
	if err != nil {
		return nil, err
	}

	// Create Windows Nodes
	var windowsInstanceID string
	windowsUserData := `<powershell>
Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
Start-Service ssh-agent; Start-Service sshd
Set-Service -Name sshd -StartupType 'Automatic'
Set-Service docker -StartUpType Disabled -Status Stopped
Stop-Process dockerd
mkdir C:\ProgramData\ssh\
Add-Content -Path C:\ProgramData\ssh\administrators_authorized_keys -Value @"
ssh-rsa %s
"@
icacls.exe "C:\ProgramData\ssh\administrators_authorized_keys" /inheritance:r /grant "Administrators:F" /grant "SYSTEM:F"
</powershell>"
`
	if hasWindows {
		runInstancesInput.UserData = aws.String(fmt.Sprintf(windowsUserData, sshKey))
		runInstancesInput, err = createNodesCommon(client, 1, hasWindows, "2019")
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

		runInstancesInput, err = createNodesCommon(client, 1, hasWindows, "2022")
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

	// wait until instance is running
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

	if hasWindows {
		pwInput := &ec2.GetPasswordDataInput{
			InstanceId: aws.String(windowsInstanceID),
		}
		// wait until password data is available to ensure the node is ready
		err = ec2Client.SVC.WaitUntilPasswordDataAvailable(pwInput)
		if err != nil {
			return nil, err
		}
	}

	// describe instance to get attributes
	describe, err := ec2Client.SVC.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: listOfInstanceIds,
	})
	if err != nil {
		return nil, err
	}

	readyInstances := describe.Reservations[0].Instances

	var ec2Nodes []*nodes.Node
	for _, readyInstance := range readyInstances {
		ec2Node := &nodes.Node{
			NodeID:          aws.StringValue(readyInstance.InstanceId),
			PublicIPAddress: aws.StringValue(readyInstance.PublicIpAddress),
			SSHUser:         ec2Client.Config.AWSUser,
			SSHKey:          sshKey,
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

func createNodesCommon(client *rancher.Client, numOfInstances int, hasWindows bool, windowsVersion string) (*ec2.RunInstancesInput, error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return nil, err
	}

	keyName := getSSHKeyName(ec2Client.Config.AWSSSHKeyName)
	if keyName == "" {
		return nil, fmt.Errorf("unable to parse AWSSSHKeyName configuration")
	}

	imageID := ec2Client.Config.AWSLinuxAMI
	instanceType := ec2Client.Config.InstanceTypeLinux
	volumeSize := ec2Client.Config.VolumeSizeLinux

	// # aws ec2 describe-images --owners amazon --filters "Name=platform,Values=windows" "Name=root-device-type,Values=ebs" "Name=name,Values=Windows*2022*Containers*"
	if hasWindows {
		var (
			windowsOwner            []string
			windows2019FilterValues []string
			windows2022FilterValues []string
		)
		windowsOwner = append(windowsOwner, "amazon")
		windowsFilter := &ec2.Filter{}

		switch windowsVersion {
		case "2019":
			windows2019FilterValues = append(windows2019FilterValues, "Name=platform,Values=windows", "Name=root-device-type,Values=ebs", "Name=name,Values=Windows*2019*Containers*")
			windowsFilter.Values = aws.StringSlice(windows2019FilterValues)
			input := ec2.DescribeImagesInput{Owners: aws.StringSlice(windowsOwner)}
			input.Filters = append(input.Filters, windowsFilter)
			images, err := ec2Client.SVC.DescribeImages(&input)
			if err != nil {
				return nil, err
			}
			img := images.Images
			sort.SliceStable(img, func(i, j int) bool {
				iCreateDate, _ := time.Parse(time.RFC3339, aws.StringValue(img[i].CreationDate))
				jCreateDate, _ := time.Parse(time.RFC3339, aws.StringValue(img[j].CreationDate))
				return iCreateDate.After(jCreateDate)
			})
			imageID = aws.StringValue(img[0].ImageId)

		case "2022":
			windows2022FilterValues = append(windows2022FilterValues, "Name=platform,Values=windows", "Name=root-device-type,Values=ebs", "Name=name,Values=Windows*2022*Containers*")
			windowsFilter.Values = aws.StringSlice(windows2022FilterValues)
			input := ec2.DescribeImagesInput{Owners: aws.StringSlice(windowsOwner)}
			input.Filters = append(input.Filters, windowsFilter)
			images, err := ec2Client.SVC.DescribeImages(&input)
			if err != nil {
				return nil, err
			}
			img := images.Images
			sort.SliceStable(img, func(i, j int) bool {
				iCreateDate, _ := time.Parse(time.RFC3339, aws.StringValue(img[i].CreationDate))
				jCreateDate, _ := time.Parse(time.RFC3339, aws.StringValue(img[j].CreationDate))
				return iCreateDate.After(jCreateDate)
			})
			imageID = aws.StringValue(img[0].ImageId)

		default:
			return nil, fmt.Errorf("windows version %s is not valid", windowsVersion)
		}

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
