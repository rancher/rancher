package ec2

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
)

const (
	nodeBaseName               = "rancherautomation"
	AutomationPemKeyName       = "automation-keypair"
	sshPath                    = ".ssh"
	defaultWindowsVolumeSize   = int(100)
	defaultWindowsInstanceType = "m5a.xlarge"
	defaultRandStringLength    = 5
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

	// Create Windows Nodes
	var windowsInstanceID string
	windowsUserData := `<powershell>\nAdd-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0\nStart-Service ssh-agent; Start-Service sshd\nSet-Service -Name sshd -StartupType 'Automatic'\nSet-Service docker -StartUpType Disabled -Status Stopped\nStop-Process dockerd\n</powershell>"`
	if hasWindows {
		runInstancesInput.UserData = aws.String(windowsUserData)
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

	sshKey, err := nodes.GetSSHKey(ec2Client.Config.AWSSSHKeyName)
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
func generatePEMKey(client *rancher.Client) (string, error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return "", err
	}
	input := ec2.CreateKeyPairInput{KeyName: aws.String(AppendRandomString(AutomationPemKeyName))}
	newKey, err := ec2Client.SVC.CreateKeyPair(&input)
	if err != nil {
		return "", err
	}
	sensitivePEM := newKey.KeyMaterial
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(filepath.Join(user.HomeDir, sshPath), 0744)
	if err != nil {
		return "", err
	}
	localPEM := filepath.Join(user.HomeDir, sshPath, aws.StringValue(input.KeyName), ".pem")
	err = os.WriteFile(localPEM, []byte(convert.ToString(sensitivePEM)), 0400)
	if err != nil {
		os.Remove(localPEM)
		return "", err
	}

	return aws.StringValue(input.KeyName), nil
}

func cleanupPEM(client *rancher.Client, keyName string) error {
	var retries = 0
	input := ec2.DeleteKeyPairInput{KeyName: aws.String(keyName)}
	err := deleteKeyPair(client, &input, retries)
	if err != nil {
		// retry once more
		retries++
		err = deleteKeyPair(client, &input, retries)
		return err
	}
	return nil
}

func deleteKeyPair(client *rancher.Client, input *ec2.DeleteKeyPairInput, retries int) error {
	if retries == 1 {
		return fmt.Errorf("unable to deleteKeyPair %s, maximum retries reached", aws.StringValue(input.KeyName))
	}
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return err
	}
	success := fmt.Sprintf("AWS KeyPair %s has been deleted successfully:\n", aws.StringValue(input.KeyName))
	result, err := ec2Client.SVC.DeleteKeyPair(input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			// Prints out full error message, including original error if there was one.
			fmt.Printf("error from AWS while trying to delete KeyPair: %s\n", awsErr.Error())
			if origErr := awsErr.OrigErr(); origErr != nil {
				// retry once more
				retries++
				fmt.Printf("retrying deletion of AWS KeyPAir %s\n", aws.StringValue(input.KeyName))
				err = deleteKeyPair(client, input, retries)
				if err == nil {
					fmt.Printf(success + result.GoString())
					return nil
				}
				return fmt.Errorf(err.Error())
			}
		}
	}
	fmt.Printf(success + result.GoString())
	return nil
}

func createNodesCommon(client *rancher.Client, numOfInstances int, hasWindows bool, windowsVersion string) (*ec2.RunInstancesInput, error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return nil, err
	}
	keyName, err := generatePEMKey(client)
	if err != nil {
		fmt.Printf("error: unable to generate runtime PEM key: %s\nattempting fallback of using AWSSSHKeyName configuration", err)
		// attempt to fallback to local sshkey config
		if getSSHKeyName(ec2Client.Config.AWSSSHKeyName) != "" {
			return nil, fmt.Errorf("unable to parse AWSSSHKeyName configuration")
		}
		fmt.Printf("using fallback AWSSSHKeyName configuration")
		keyName = getSSHKeyName(ec2Client.Config.AWSSSHKeyName)
	}

	imageID := ec2Client.Config.AWSLinuxAMI
	instanceType := ec2Client.Config.InstanceTypeLinux
	volumeSize := ec2Client.Config.VolumeSizeLinux

	// # aws ec2 describe-images --owners amazon --filters "Name=platform,Values=windows" "Name=root-device-type,Values=ebs" "Name=name,Values=Windows*2019*Containers*"
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

func AppendRandomString(baseClusterName string) string {
	clusterName := "auto-" + baseClusterName + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	return clusterName
}
