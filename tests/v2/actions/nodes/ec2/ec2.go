package ec2

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	rancherEc2 "github.com/rancher/shepherd/clients/ec2"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/nodes"
)

// CreateNodes creates `quantityPerPool[n]` number of ec2 instances
func CreateNodes(client *rancher.Client, rolesPerPool []string, quantityPerPool []int32) (ec2Nodes []*nodes.Node, err error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return nil, err
	}

	runningReservations := []*ec2.Reservation{}
	reservationConfigs := []*rancherEc2.AWSEC2Config{}
	// provisioning instances in reverse order to allow windows instances time to become ready
	for i := len(quantityPerPool) - 1; i >= 0; i-- {
		config := MatchRoleToConfig(rolesPerPool[i], ec2Client.ClientConfig.AWSEC2Config)
		if config == nil {
			return nil, errors.New("No matching nodesAndRole for AWSEC2Config with role:" + rolesPerPool[i])
		}

		nodeBaseName := config.AWSCICDInstanceTag
		if nodeBaseName == "" {
			nodeBaseName = "rancher-qa-automation"
		}
		sshName := getSSHKeyName(config.AWSSSHKeyName)
		runInstancesInput := &ec2.RunInstancesInput{
			ImageId:      aws.String(config.AWSAMI),
			InstanceType: aws.String(config.InstanceType),
			MinCount:     aws.Int64(int64(quantityPerPool[i])),
			MaxCount:     aws.Int64(int64(quantityPerPool[i])),
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
							Value: aws.String(nodeBaseName + namegenerator.RandStringLower(5)),
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
			return nil, err
		}
		// instead of waiting on each node pool to complete provisioning, add to a queue and check run status later
		runningReservations = append(runningReservations, reservation)
		reservationConfigs = append(reservationConfigs, config)
	}

	for i := 0; i < len(quantityPerPool); i++ {
		var listOfInstanceIds []*string

		for _, instance := range runningReservations[i].Instances {
			listOfInstanceIds = append(listOfInstanceIds, instance.InstanceId)
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

		// describe instance to get attributes
		describe, err := ec2Client.SVC.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: listOfInstanceIds,
		})
		if err != nil {
			return nil, err
		}

		readyInstances := describe.Reservations[0].Instances

		sshKey, err := nodes.GetSSHKey(reservationConfigs[i].AWSSSHKeyName)
		if err != nil {
			return nil, err
		}

		for _, readyInstance := range readyInstances {
			ec2Node := &nodes.Node{
				NodeID:           *readyInstance.InstanceId,
				PublicIPAddress:  *readyInstance.PublicIpAddress,
				PrivateIPAddress: *readyInstance.PrivateIpAddress,
				SSHUser:          reservationConfigs[i].AWSUser,
				SSHKey:           sshKey,
			}
			// re-reverse the list so that the order is corrected
			ec2Nodes = append([]*nodes.Node{ec2Node}, ec2Nodes...)
		}
	}

	client.Session.RegisterCleanupFunc(func() error {
		return DeleteNodes(client, ec2Nodes)
	})

	return ec2Nodes, nil
}

// MatchRoleToConfig matches the role of nodesAndRoles to the ec2Config that allows this role.
func MatchRoleToConfig(poolRole string, ec2Configs []rancherEc2.AWSEC2Config) *rancherEc2.AWSEC2Config {
	for _, config := range ec2Configs {
		hasMatch := false
		for _, configRole := range config.Roles {
			if strings.Contains(poolRole, configRole) {
				hasMatch = true
			}
		}
		if hasMatch {
			return &config
		}
	}
	return nil
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
