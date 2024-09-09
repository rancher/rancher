package clusterupstreamrefresher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekscontroller "github.com/rancher/eks-operator/controller"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	eksUpstreamRefresh       = "eks-refresh"
	eksRefreshCronDeprecated = "eks-refresh-cron"
	eksRefreshCronAnnotation = "settings.management.cattle.io/migrated"
)

func BuildEKSUpstreamSpec(secretClient wranglerv1.SecretClient, cluster *mgmtv3.Cluster) (*eksv1.EKSClusterConfigSpec, error) {
	ctx := context.Background()
	eksService, err := ekscontroller.StartEKSService(ctx, secretClient, *cluster.Spec.EKSConfig)
	if err != nil {
		return nil, err
	}

	clusterState, err := eksService.DescribeCluster(ctx,
		&eks.DescribeClusterInput{
			Name: aws.String(cluster.Spec.EKSConfig.DisplayName),
		})
	if err != nil {
		return nil, err
	}

	ngs, err := eksService.ListNodegroups(ctx,
		&eks.ListNodegroupsInput{
			ClusterName: aws.String(cluster.Spec.EKSConfig.DisplayName),
		})
	if err != nil {
		return nil, err
	}

	// gather upstream node groups states
	var nodeGroupStates []*eks.DescribeNodegroupOutput
	var errs []string
	for _, ngName := range ngs.Nodegroups {
		ng, err := eksService.DescribeNodegroup(ctx,
			&eks.DescribeNodegroupInput{
				ClusterName:   aws.String(cluster.Spec.EKSConfig.DisplayName),
				NodegroupName: aws.String(ngName),
			})
		if err != nil {
			return nil, err
		}

		nodeGroupStates = append(nodeGroupStates, ng)
		var nodeGroupMustBeDeleted string
		if len(ng.Nodegroup.Health.Issues) != 0 {
			var issueMessages []string
			for _, issue := range ng.Nodegroup.Health.Issues {
				issueMessages = append(issueMessages, aws.ToString(issue.Message))
				if !ekscontroller.NodeGroupIssueIsUpdatable(string(issue.Code)) {
					nodeGroupMustBeDeleted = ": node group cannot be updated, must be deleted and recreated"
				}
			}
			errs = append(errs, fmt.Sprintf("health error for node group [%s] in cluster [%s]: %s%s",
				aws.ToString(ng.Nodegroup.NodegroupName),
				cluster.Name,
				strings.Join(issueMessages, "; "),
				nodeGroupMustBeDeleted,
			))
		}
	}

	ec2Service, err := ekscontroller.StartEC2Service(ctx, secretClient, *cluster.Spec.EKSConfig)
	if err != nil {
		return nil, err
	}
	upstreamSpec, _, err := ekscontroller.BuildUpstreamClusterState(ctx, cluster.Spec.DisplayName, cluster.Status.EKSStatus.ManagedLaunchTemplateID, clusterState, nodeGroupStates, ec2Service, eksService, false)
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

// MigrateEksRefreshCronSetting migrates the deprecated eks-refresh-cron setting to new
// setting only if default setting was changed
// This function will be run only once during startup by pkg/multiclustermanager/app.go
func MigrateEksRefreshCronSetting(wContext *wrangler.Context) {
	settingsClient := wContext.Mgmt.Setting()
	eksCronSetting, err := settingsClient.Get(eksRefreshCronDeprecated, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return
	} else if err != nil {
		logrus.Errorf("Unable to complete EKS cron migration, will attempt at next rancher startup. "+
			"Error getting %s setting: %v", eksRefreshCronDeprecated, err)
		return
	}
	if eksCronSetting.Annotations != nil && eksCronSetting.Annotations[eksRefreshCronAnnotation] == "true" {
		return
	}

	eksCronAnnotate := make(map[string]string)
	if eksCronSetting.Annotations != nil {
		eksCronAnnotate = eksCronSetting.Annotations
	}
	eksCronAnnotate[eksRefreshCronAnnotation] = "true"

	settingsClientCache := wContext.Mgmt.Setting().Cache()
	eksRefreshSetting, err := settingsClientCache.Get(eksUpstreamRefresh)
	if errors.IsNotFound(err) {
		return
	} else if err != nil {
		logrus.Errorf("Unable to complete EKS cron migration, will attempt at next rancher startup. "+
			"Error getting %s setting: %v", eksUpstreamRefresh, err)
		return
	}

	if eksRefreshSetting.Value != "" || eksCronSetting.Value == "" {
		eksCronSetting.SetAnnotations(eksCronAnnotate)
		if _, err = settingsClient.Update(eksCronSetting); err != nil {
			logrus.Errorf("Unable to complete EKS cron migration, will attempt at next rancher startup. "+
				"Error annotating eks-refresh-cron setting: %v", err)
		}
		return
	}

	eksSchedule, err := cron.ParseStandard(eksCronSetting.Value)
	if err != nil {
		logrus.Errorf("Unable to complete EKS cron migration, will attempt at next rancher startup. "+
			"Error parsing cron schedule %s setting: %v", eksRefreshCronDeprecated, err)
		return
	}

	next := eksSchedule.Next(time.Now())
	refreshTime := int(eksSchedule.Next(next).Sub(next) / time.Second)

	err = settings.EKSUpstreamRefresh.Set(fmt.Sprint(refreshTime))
	if err != nil {
		logrus.Errorf("Unable to complete EKS cron migration, will attempt at next rancher startup. "+
			"Error updating eks-refresh setting: %v", err)
	}
	eksCronSetting.SetAnnotations(eksCronAnnotate)
	if _, err = settingsClient.Update(eksCronSetting); err != nil {
		logrus.Errorf("Unable to complete EKS cron migration, will attempt at next rancher startup. "+
			"Error annotating eks-refresh-cron setting: %v", err)
	}
}
