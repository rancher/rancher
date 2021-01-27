package eksupstreamrefresh

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/rancher/eks-operator/controller"
	v1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	isEKSIndexer = "clusters.management.cattle.io/is-eks"
)

var (
	eksUpstreamRefresher *eksRefreshController
)

func init() {
	// possible settings controller, which references refresh
	// cron job, will run prior to StartEKSUpstreamCronJob.
	// This ensure the CronJob will not be nil
	eksUpstreamRefresher = &eksRefreshController{
		refreshCronJob: cron.New(),
	}
}

type eksRefreshController struct {
	refreshCronJob *cron.Cron
	secretsCache   wranglerv1.SecretCache
	clusterClient  v3.ClusterClient
	clusterCache   v3.ClusterCache
}

func StartEKSUpstreamCronJob(wContext *wrangler.Context) {
	eksUpstreamRefresher.secretsCache = wContext.Core.Secret().Cache()
	eksUpstreamRefresher.clusterClient = wContext.Mgmt.Cluster()
	eksUpstreamRefresher.clusterCache = wContext.Mgmt.Cluster().Cache()

	eksUpstreamRefresher.clusterCache.AddIndexer(isEKSIndexer, func(obj *apimgmtv3.Cluster) ([]string, error) {
		if obj.Spec.EKSConfig == nil {
			return []string{}, nil
		}
		return []string{"true"}, nil
	})

	schedule, err := cron.ParseStandard(settings.EKSUpstreamRefreshCron.Get())
	if err != nil {
		logrus.Errorf("Error parsing EKS upstream cluster refresh cron. Upstream state will not be refreshed: %v", err)
		return
	}
	eksUpstreamRefresher.refreshCronJob.Schedule(schedule, cron.FuncJob(eksUpstreamRefresher.refreshAllUpstreamStates))
	eksUpstreamRefresher.refreshCronJob.Start()
}

func (e *eksRefreshController) refreshAllUpstreamStates() {
	logrus.Debugf("Refreshing EKS clusters' upstream states")
	clusters, err := e.clusterCache.GetByIndex(isEKSIndexer, "true")
	if err != nil {
		logrus.Error("error trying to refresh EKS clusters' upstream states")
		return
	}

	for _, cluster := range clusters {
		if _, err := e.refreshClusterUpstreamSpec(cluster); err != nil {
			logrus.Errorf("error refreshing EKS cluster [%s] upstream state", cluster.Name)
		}
	}
}

func (e *eksRefreshController) refreshClusterUpstreamSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	if cluster.Spec.EKSConfig == nil {
		return cluster, nil
	}

	logrus.Infof("checking cluster [%s] upstream state for changes", cluster.Name)

	if cluster.Status.EKSStatus.UpstreamSpec == nil {
		logrus.Infof("initial upstream spec for cluster [%s] has not been set by eks cluster handler yet, skipping", cluster.Name)
		return cluster, nil
	}

	// In this call, it is possible to get errors back with non-nil upstreamSpec.
	// If upstreamSpec is nil then the syncing failed for some reason. This is reported to the user, and this function returns at the end of this if-statement.
	// If upstreamSpec is non-nil then the syncing occurred as expected, but the node groups have health issues that are reported to the user.
	// In this second case, the message is set on the Updated condition, but execution continues because the sync was successful.
	upstreamSpec, err := GetComparableUpstreamSpec(e.secretsCache, cluster)
	if err != nil {
		var statusErr error
		var syncFailed string
		if upstreamSpec == nil {
			syncFailed = ": syncing failed"
		}
		cluster = cluster.DeepCopy()
		apimgmtv3.ClusterConditionUpdated.False(cluster)
		apimgmtv3.ClusterConditionUpdated.Message(cluster, fmt.Sprintf("[Syncing error%s] %s", syncFailed, err.Error()))

		cluster, statusErr = e.clusterClient.Update(cluster)
		if statusErr != nil {
			return cluster, statusErr
		}

		if upstreamSpec == nil {
			return cluster, err
		}
	} else if strings.Contains(apimgmtv3.ClusterConditionUpdated.GetMessage(cluster), "[Syncing error") {
		cluster = cluster.DeepCopy()
		apimgmtv3.ClusterConditionUpdated.True(cluster)
		apimgmtv3.ClusterConditionUpdated.Message(cluster, "")
		cluster, err = e.clusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	if !reflect.DeepEqual(cluster.Status.EKSStatus.UpstreamSpec, upstreamSpec) {
		logrus.Infof("updating cluster [%s], upstream change detected", cluster.Name)
		cluster = cluster.DeepCopy()
		cluster.Status.EKSStatus.UpstreamSpec = upstreamSpec
		cluster, err = e.clusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	if !reflect.DeepEqual(cluster.Spec.EKSConfig, cluster.Status.AppliedSpec.EKSConfig) {
		logrus.Infof("cluster [%s] currently updating, skipping spec sync", cluster.Name)
		return cluster, nil
	}

	// check for changes between EKS spec on cluster and the EKS spec on the EKSClusterConfig object

	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cluster.Spec.EKSConfig)
	if err != nil {
		return cluster, err
	}

	upstreamSpecMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(upstreamSpec)
	if err != nil {
		return cluster, err
	}

	var updateEKSConfig bool
	for key, value := range upstreamSpecMap {
		if specMap[key] == nil {
			continue
		}
		if reflect.DeepEqual(specMap[key], value) {
			continue
		}
		updateEKSConfig = true
		specMap[key] = value
	}

	if !updateEKSConfig {
		logrus.Infof("cluster [%s] matches upstream, skipping spec sync", cluster.Name)
		return cluster, nil
	}

	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.EKSConfig); err != nil {
		return cluster, err
	}

	return e.clusterClient.Update(cluster)
}

func GetComparableUpstreamSpec(secretsCache wranglerv1.SecretCache, cluster *mgmtv3.Cluster) (*v1.EKSClusterConfigSpec, error) {
	switch cluster.Status.Driver {
	case apimgmtv3.ClusterDriverEKS:
		return buildEKSUpstreamSpec(secretsCache, cluster)
	default:
		return nil, fmt.Errorf("unsupported cloud driver")
	}
}

func buildEKSUpstreamSpec(secretsCache wranglerv1.SecretCache, cluster *mgmtv3.Cluster) (*v1.EKSClusterConfigSpec, error) {
	sess, eksService, err := controller.StartAWSSessions(secretsCache, *cluster.Spec.EKSConfig)
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
				if !controller.NodeGroupIssueIsUpdatable(aws.StringValue(issue.Code)) {
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

	upstreamSpec, _, err := controller.BuildUpstreamClusterState(cluster.Spec.DisplayName, cluster.Status.EKSStatus.ManagedLaunchTemplateID, clusterState, nodeGroupStates, ec2.New(sess), false)
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
