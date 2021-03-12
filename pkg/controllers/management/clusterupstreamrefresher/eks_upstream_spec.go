package clusterupstreamrefresher

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	ekscontroller "github.com/rancher/eks-operator/controller"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
)

func BuildEKSUpstreamSpec(secretsCache wranglerv1.SecretCache, cluster *mgmtv3.Cluster) (*eksv1.EKSClusterConfigSpec, error) {
	sess, eksService, err := ekscontroller.StartAWSSessions(secretsCache, *cluster.Spec.EKSConfig)
	if err != nil {
		return nil, err
	}

	clusterState, err := eksService.DescribeCluster(
		&eks.DescribeClusterInput{
			Name: aws.String(cluster.Spec.EKSConfig.DisplayName),
		})
	if err != nil {
		return nil, err
	}

	ngs, err := eksService.ListNodegroups(
		&eks.ListNodegroupsInput{
			ClusterName: aws.String(cluster.Spec.EKSConfig.DisplayName),
		})

	// gather upstream node groups states
	var nodeGroupStates []*eks.DescribeNodegroupOutput
	var errs []string
	for _, ngName := range ngs.Nodegroups {
		ng, err := eksService.DescribeNodegroup(
			&eks.DescribeNodegroupInput{
				ClusterName:   aws.String(cluster.Spec.EKSConfig.DisplayName),
				NodegroupName: ngName,
			})
		if err != nil {
			return nil, err
		}

		nodeGroupStates = append(nodeGroupStates, ng)
		var nodeGroupMustBeDeleted string
		if len(ng.Nodegroup.Health.Issues) != 0 {
			var issueMessages []string
			for _, issue := range ng.Nodegroup.Health.Issues {
				issueMessages = append(issueMessages, aws.StringValue(issue.Message))
				if !ekscontroller.NodeGroupIssueIsUpdatable(aws.StringValue(issue.Code)) {
					nodeGroupMustBeDeleted = ": node group cannot be updated, must be deleted and recreated"
				}
			}
			errs = append(errs, fmt.Sprintf("health error for node group [%s] in cluster [%s]: %s%s",
				aws.StringValue(ng.Nodegroup.NodegroupName),
				cluster.Name,
				strings.Join(issueMessages, "; "),
				nodeGroupMustBeDeleted,
			))
		}
	}

	upstreamSpec, _, err := ekscontroller.BuildUpstreamClusterState(cluster.Spec.DisplayName, cluster.Status.EKSStatus.ManagedLaunchTemplateID, clusterState, nodeGroupStates, ec2.New(sess), false)
	if err != nil {
		// If we get an error here, then syncing is broken
		return nil, err
	}

	upstreamSpec.DisplayName = cluster.Spec.EKSConfig.DisplayName
	upstreamSpec.Region = cluster.Spec.EKSConfig.Region
	upstreamSpec.AmazonCredentialSecret = cluster.Spec.EKSConfig.AmazonCredentialSecret
	upstreamSpec.Imported = cluster.Spec.EKSConfig.Imported
	upstreamSpec.Subnets = cluster.Spec.EKSConfig.Subnets
	upstreamSpec.SecurityGroups = cluster.Spec.EKSConfig.SecurityGroups
	upstreamSpec.ServiceRole = cluster.Spec.EKSConfig.ServiceRole

	if len(errs) != 0 {
		// If there are errors here, we can still sync, but there are problems with the nodegroups that should be reported
		err = fmt.Errorf("error for cluster [%s]: %s",
			cluster.Name,
			strings.Join(errs, "\n"))
	}

	return upstreamSpec, err
}
