package ec2

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/pkg/nodes"
)

const (
	nodeBaseName = "rancher-automation"
)

// CreateNodes creates `numOfInstances` (and/or `numOfWinInstances` when using multiple node configurations - e.g. Windows nodes) number of ec2 instances
func CreateNodes(client *rancher.Client, numOfInstances int, numOfWinInstances int, multiconfig bool) (ec2Nodes []*nodes.Node, winEC2Nodes []*nodes.Node, err error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return nil, nil, err
	}

	for _, config := range ec2Client.ClientConfig.AWSEC2Config {
		sshName := getSSHKeyName(config.AWSSSHKeyName)
		runInstancesInput := &ec2.RunInstancesInput{
			ImageId:      aws.String(config.AWSAMI),
			InstanceType: aws.String(config.InstanceType),
			MinCount:     aws.Int64(int64(numOfInstances)),
			MaxCount:     aws.Int64(int64(numOfInstances)),
			KeyName:      aws.String(sshName),
			BlockDeviceMappings: []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda1"),
					Ebs: &ec2.EbsBlockDevice{
						VolumeSize: aws.Int64(int64(config.VolumeSize)),
					},
				},
			},
			IamInstanceProfile: &ec2.IamInstanceProfileSpecification{
				Name: aws.String(config.AWSIAMProfile),
			},
			Placement: &ec2.Placement{
				AvailabilityZone: aws.String(config.AWSRegionAZ),
			},
			NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
				{
					DeviceIndex:              aws.Int64(0),
					AssociatePublicIpAddress: aws.Bool(true),
					Groups:                   aws.StringSlice(config.AWSSecurityGroups),
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
							Value: aws.String(config.AWSCICDInstanceTag),
						},
					},
				},
			},
		}

		reservation, err := ec2Client.SVC.RunInstances(runInstancesInput)
		if err != nil {
			return nil, nil, err
		}

		var listOfInstanceIds []*string

		for _, instance := range reservation.Instances {
			listOfInstanceIds = append(listOfInstanceIds, instance.InstanceId)
		}

		//wait until instance is running
		err = ec2Client.SVC.WaitUntilInstanceRunning(&ec2.DescribeInstancesInput{
			InstanceIds: listOfInstanceIds,
		})
		if err != nil {
			return nil, nil, err
		}

		//wait until instance status is ok
		err = ec2Client.SVC.WaitUntilInstanceStatusOk(&ec2.DescribeInstanceStatusInput{
			InstanceIds: listOfInstanceIds,
		})
		if err != nil {
			return nil, nil, err
		}

		// describe instance to get attributes=
		describe, err := ec2Client.SVC.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: listOfInstanceIds,
		})
		if err != nil {
			return nil, nil, err
		}

		readyInstances := describe.Reservations[0].Instances

		sshKey, err := nodes.GetSSHKey(config.AWSSSHKeyName)
		if err != nil {
			return nil, nil, err
		}

		for _, readyInstance := range readyInstances {

			if multiconfig {
				if config.AWSAMI == ec2Client.ClientConfig.AWSEC2Config[0].AWSAMI &&
					config.AWSUser == ec2Client.ClientConfig.AWSEC2Config[0].AWSUser {
					ec2Node := &nodes.Node{
						NodeID:          *readyInstance.InstanceId,
						PublicIPAddress: *readyInstance.PublicIpAddress,
						SSHUser:         config.AWSUser,
						SSHKey:          sshKey,
					}
					ec2Nodes = append(ec2Nodes, ec2Node)
				}

				if config.AWSAMI == ec2Client.ClientConfig.AWSEC2Config[1].AWSAMI &&
					config.AWSUser == ec2Client.ClientConfig.AWSEC2Config[1].AWSUser {
					ec2Node2 := &nodes.Node{
						NodeID:          *readyInstance.InstanceId,
						PublicIPAddress: *readyInstance.PublicIpAddress,
						SSHUser:         config.AWSUser,
						SSHKey:          sshKey,
					}
					winEC2Nodes = append(winEC2Nodes, ec2Node2)
				}
			} else {
				ec2Node := &nodes.Node{
					NodeID:          *readyInstance.InstanceId,
					PublicIPAddress: *readyInstance.PublicIpAddress,
					SSHUser:         config.AWSUser,
					SSHKey:          sshKey,
				}
				ec2Nodes = append(ec2Nodes, ec2Node)
				winEC2Nodes = nil
			}
		}

		client.Session.RegisterCleanupFunc(func() error {
			return DeleteNodes(client, ec2Nodes, winEC2Nodes, multiconfig)
		})

		if !multiconfig {
			break
		}
	}

	return ec2Nodes, winEC2Nodes, nil
}

// DeleteNodes terminates ec2 instances that have been created.
func DeleteNodes(client *rancher.Client, nodes []*nodes.Node, nodes2 []*nodes.Node, multiconfig bool) error {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return err
	}

	var instanceIDs []*string
	for _, node := range nodes {
		instanceIDs = append(instanceIDs, aws.String(node.NodeID))
	}
	if multiconfig {
		for _, node2 := range nodes2 {
			instanceIDs = append(instanceIDs, aws.String(node2.NodeID))
		}
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
