package nodes

import (
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	active = "active"
)

// IsNodeReady is a helper method that will loop and check if the node is ready in the RKE1 cluster.
// It will return an error if the node is not ready after set amount of time.
func IsNodeReady(client *rancher.Client, ClusterID string) error {
	err := wait.Poll(500*time.Millisecond, 30*time.Minute, func() (bool, error) {
		nodes, err := client.Management.Node.ListAll(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": ClusterID,
			},
		})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Data {
			node, err := client.Management.Node.ByID(node.ID)
			if err != nil {
				return false, err
			}

			if node.State == active {
				logrus.Infof("All nodes in the cluster are in an active state!")
				return true, nil
			}

			return false, nil
		}

		return false, nil
	})

	return err
}

// IsNodeDeleted checks if the node is in terminated state or does not exist.
// it throws an error if multiple nodes with the same name exist.
func IsNodeDeleted(client *rancher.Client, nodeName string) (bool, error) {
	ec2Client, err := client.GetEC2Client()
	if err != nil {
		return false, err
	}

	resp, err := ec2Client.SVC.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("Name"),
				Values: []*string{
					aws.String(nodeName),
				},
			},
		},
	})

	if len(resp.Reservations) == 0 {
		return true, nil
	}

	instances := []*ec2.Instance{}

	for _, reservation := range resp.Reservations {
		instances = append(instances, reservation.Instances...)
	}

	if len(instances) == 0 {
		return true, nil
	}

	if len(instances) > 1 {
		return false, errors.New("multiple instances with the same name exist")
	}

	if instances[0].State.Name != nil && *instances[0].State.Name == ec2.InstanceStateNameTerminated {
		return true, nil
	}

	return false, err
}
