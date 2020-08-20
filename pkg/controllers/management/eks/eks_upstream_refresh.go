package eks

import (
	"context"
	"reflect"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/rancher/eks-operator/controller"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type eksRefreshController struct {
	clusterEnqueueAfter  func(name string, duration time.Duration)
	secretsCache         wranglerv1.SecretCache
	clusterClient        v3.ClusterClient
	systemAccountManager *systemaccount.Manager
	dynamicClient        dynamic.NamespaceableResourceInterface
}

func RegisterRefresh(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext) {
	eksClusterConfigResource := schema.GroupVersionResource{
		Group:    eksAPIGroup,
		Version:  "v1",
		Resource: "eksclusterconfigs",
	}

	eksCCDynamicClient := mgmtCtx.DynamicClient.Resource(eksClusterConfigResource)
	e := &eksRefreshController{
		clusterEnqueueAfter:  wContext.Mgmt.Cluster().EnqueueAfter,
		secretsCache:         wContext.Core.Secret().Cache(),
		clusterClient:        wContext.Mgmt.Cluster(),
		systemAccountManager: systemaccount.NewManager(mgmtCtx),
		dynamicClient:        eksCCDynamicClient,
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "eks-refresh-controller", e.onClusterChange)
}

func (e *eksRefreshController) onClusterChange(key string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	if cluster.Spec.EKSConfig == nil {
		return cluster, nil
	}

	logrus.Infof("checking cluster [%s] upstream state for changes", cluster.Name)

	_, eksService, err := controller.StartAWSSessions(e.secretsCache, *cluster.Spec.EKSConfig)
	if err != nil {
		return cluster, err
	}

	clusterState, err := eksService.DescribeCluster(
		&eks.DescribeClusterInput{
			Name: aws.String(cluster.Spec.EKSConfig.DisplayName),
		})
	if err != nil {
		if notFound(err) {
			return cluster, nil
		}
		return cluster, err
	}

	ngs, err := eksService.ListNodegroups(
		&eks.ListNodegroupsInput{
			ClusterName: aws.String(cluster.Spec.EKSConfig.DisplayName),
		})

	// gather upstream node groups states
	var nodeGroupStates []*eks.DescribeNodegroupOutput
	for _, ngName := range ngs.Nodegroups {
		ng, err := eksService.DescribeNodegroup(
			&eks.DescribeNodegroupInput{
				ClusterName:   aws.String(cluster.Spec.EKSConfig.DisplayName),
				NodegroupName: ngName,
			})
		if err != nil {
			return cluster, err
		}

		nodeGroupStates = append(nodeGroupStates, ng)
	}

	upstreamSpec, _, err := controller.BuildUpstreamClusterState(cluster.Spec.DisplayName, clusterState, nodeGroupStates, eksService)
	if err != nil {
		return cluster, err
	}

	upstreamSpec.DisplayName = cluster.Spec.EKSConfig.DisplayName
	upstreamSpec.Region = cluster.Spec.EKSConfig.Region
	upstreamSpec.AmazonCredentialSecret = cluster.Spec.EKSConfig.AmazonCredentialSecret
	upstreamSpec.Imported = cluster.Spec.EKSConfig.Imported
	upstreamSpec.Subnets = cluster.Spec.EKSConfig.Subnets
	upstreamSpec.SecurityGroups = cluster.Spec.EKSConfig.SecurityGroups
	upstreamSpec.ServiceRole = cluster.Spec.EKSConfig.ServiceRole

	if cluster.Status.EKSStatus.UpstreamSpec == nil {
		logrus.Infof("setting initial upstream spec for cluster [%s]", cluster.Name)
		cluster = cluster.DeepCopy()
		cluster.Status.EKSStatus.UpstreamSpec = upstreamSpec
		return e.clusterClient.Update(cluster)
	}

	if !reflect.DeepEqual(cluster.Status.EKSStatus.UpstreamSpec, upstreamSpec) {
		logrus.Infof("updating cluster [%s], upstream change detected", cluster.Name)
		cluster = cluster.DeepCopy()
		cluster.Status.EKSStatus.UpstreamSpec = upstreamSpec
		return e.clusterClient.Update(cluster)
	}

	if !reflect.DeepEqual(cluster.Spec.EKSConfig, cluster.Status.AppliedSpec.EKSConfig) {
		logrus.Infof("cluster [%s] currently updating, skipping spec sync", cluster.Name)
		e.clusterEnqueueAfter(cluster.Name, 5*time.Minute)
		return cluster, nil
	}

	// check for changes between EKS spec on cluster and the EKS spec on the EKSClusterConfig object

	if reflect.DeepEqual(*upstreamSpec, *cluster.Spec.EKSConfig) {
		logrus.Infof("cluster [%s] matches upstream, skipping spec sync", cluster.Name)
		e.clusterEnqueueAfter(cluster.Name, 5*time.Minute)
		return cluster, nil
	}

	specMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cluster.Spec.EKSConfig)
	if err != nil {
		return cluster, err
	}

	upstreamSpecMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(upstreamSpec)
	if err != nil {
		return cluster, err
	}

	for key, value := range upstreamSpecMap {
		if specMap[key] != nil {
			specMap[key] = value
		}
	}

	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(specMap, cluster.Spec.EKSConfig); err != nil {
		return cluster, err
	}

	return e.clusterClient.Update(cluster)
}

func notFound(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceNotFoundException
	}

	return false
}
