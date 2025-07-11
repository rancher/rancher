package cluster

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type Formatter struct {
	KontainerDriverLister     v3.KontainerDriverLister
	nodeLister                v3.NodeLister
	clusterLister             v3.ClusterLister
	SubjectAccessReviewClient v1.SubjectAccessReviewInterface
}

func NewFormatter(schemas *types.Schemas, managementContext *config.ScaledContext) *Formatter {
	clusterFormatter := Formatter{
		KontainerDriverLister:     managementContext.Management.KontainerDrivers("").Controller().Lister(),
		nodeLister:                managementContext.Management.Nodes("").Controller().Lister(),
		clusterLister:             managementContext.Management.Clusters("").Controller().Lister(),
		SubjectAccessReviewClient: managementContext.K8sClient.AuthorizationV1().SubjectAccessReviews(),
	}
	return &clusterFormatter
}

func (f *Formatter) Formatter(request *types.APIContext, resource *types.RawResource) {
	if convert.ToBool(resource.Values["internal"]) {
		delete(resource.Links, "remove")
	}
	shellLink := request.URLBuilder.Link("shell", resource)
	shellLink = strings.Replace(shellLink, "http", "ws", 1)
	shellLink = strings.Replace(shellLink, "/shell", "?shell=true", 1)
	resource.Links["shell"] = shellLink
	resource.AddAction(request, v32.ClusterActionGenerateKubeconfig)
	resource.AddAction(request, v32.ClusterActionImportYaml)

	if gkeConfig, ok := resource.Values["googleKubernetesEngineConfig"]; ok && gkeConfig != nil {
		configMap, ok := gkeConfig.(map[string]interface{})
		if !ok {
			logrus.Errorf("could not convert gke config to map")
			return
		}

		setTrueIfNil(configMap, "enableStackdriverLogging")
		setTrueIfNil(configMap, "enableStackdriverMonitoring")
		setTrueIfNil(configMap, "enableHorizontalPodAutoscaling")
		setTrueIfNil(configMap, "enableHttpLoadBalancing")
		setTrueIfNil(configMap, "enableNetworkPolicyConfig")
	}

	if eksConfig, ok := resource.Values["amazonElasticContainerServiceConfig"]; ok && eksConfig != nil {
		configMap, ok := eksConfig.(map[string]interface{})
		if !ok {
			logrus.Errorf("could not convert eks config to map")
			return
		}

		setTrueIfNil(configMap, "associateWorkerNodePublicIp")
		setIntIfNil(configMap, "nodeVolumeSize", 20)
	}

	nodes, err := f.nodeLister.List(resource.ID, labels.Everything())
	if err != nil {
		logrus.Warnf("error getting node list for cluster %s: %s", resource.ID, err)
	} else {
		resource.Values["nodeCount"] = len(nodes)
	}
}

func setTrueIfNil(configMap map[string]interface{}, fieldName string) {
	if configMap[fieldName] == nil {
		configMap[fieldName] = true
	}
}

func setIntIfNil(configMap map[string]interface{}, fieldName string, replaceVal int) {
	if configMap[fieldName] == nil {
		configMap[fieldName] = replaceVal
	}
}

func (f *Formatter) CollectionFormatter(request *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(request, "createFromTemplate")
}
