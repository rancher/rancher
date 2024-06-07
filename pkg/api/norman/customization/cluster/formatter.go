package cluster

import (
	"strings"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	client "github.com/rancher/rancher/pkg/client/generated/management/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	managementschema "github.com/rancher/rancher/pkg/schemas/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type Formatter struct {
	KontainerDriverLister     v3.KontainerDriverLister
	nodeLister                v3.NodeLister
	clusterLister             v3.ClusterLister
	clusterSpecPwdFields      map[string]interface{}
	SubjectAccessReviewClient v1.SubjectAccessReviewInterface
}

func NewFormatter(schemas *types.Schemas, managementContext *config.ScaledContext) *Formatter {
	clusterFormatter := Formatter{
		KontainerDriverLister:     managementContext.Management.KontainerDrivers("").Controller().Lister(),
		nodeLister:                managementContext.Management.Nodes("").Controller().Lister(),
		clusterLister:             managementContext.Management.Clusters("").Controller().Lister(),
		clusterSpecPwdFields:      gatherClusterSpecPwdFields(schemas, schemas.Schema(&managementschema.Version, client.ClusterSpecBaseType)),
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

	// If this is an RKE1 cluster only
	if _, ok := resource.Values["rancherKubernetesEngineConfig"]; ok {
		resource.AddAction(request, v32.ClusterActionExportYaml)

		// If a user has the backupetcd role/privilege, add it- In this case, the resource is the cluster, so use
		// the ID as the namespace for the ETCD check since that's where the backups live
		if _, ok := values.GetValue(resource.Values, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig"); ok && canBackupEtcd(request, resource.ID) {
			resource.AddAction(request, v32.ClusterActionBackupEtcd)
		}

		// If user has permissions to update the cluster
		if canUpdateClusterWithValues(request, resource.Values) {
			if _, ok := values.GetValue(resource.Values, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig"); ok {
				resource.AddAction(request, v32.ClusterActionRestoreFromEtcdBackup)
			}
			resource.AddAction(request, v32.ClusterActionRotateCertificates)
			if rotateEncryptionKeyEnabled(f.clusterLister, resource.ID) {
				resource.AddAction(request, v32.ClusterActionRotateEncryptionKey)
			}

			if val, ok := values.GetValue(resource.Values, "clusterTemplateRevisionId"); ok && val == nil {
				if err := request.AccessControl.CanDo(v3.ClusterTemplateGroupVersionKind.Group, v3.ClusterTemplateResource.Name, "create", request, resource.Values, request.Schema); err == nil {
					resource.AddAction(request, v32.ClusterActionSaveAsTemplate)
				}
			}
		}

	}

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

	if clusterTemplateAnswers, ok := resource.Values["answers"]; ok {
		answerMap := convert.ToMapInterface(convert.ToMapInterface(clusterTemplateAnswers)["values"])
		hideClusterTemplateAnswers(answerMap, f.clusterSpecPwdFields)

		appliedAnswers := values.GetValueN(resource.Values, "appliedSpec", "answers")

		if appliedAnswers != nil {
			appliedAnswerMap := convert.ToMapInterface(convert.ToMapInterface(appliedAnswers)["values"])
			hideClusterTemplateAnswers(appliedAnswerMap, f.clusterSpecPwdFields)
		}

		failedAnswers := values.GetValueN(resource.Values, "failedSpec", "answers")

		if failedAnswers != nil {
			failedAnswerMap := convert.ToMapInterface(convert.ToMapInterface(failedAnswers)["values"])
			hideClusterTemplateAnswers(failedAnswerMap, f.clusterSpecPwdFields)
		}
	}

	nodes, err := f.nodeLister.List(resource.ID, labels.Everything())
	if err != nil {
		logrus.Warnf("error getting node list for cluster %s: %s", resource.ID, err)
	} else {
		resource.Values["nodeCount"] = len(nodes)
	}
}

// rotateEncryptionKeyEnabled returns true if the rotateEncryptionKey action should be enabled in the API view, otherwise, it returns false.
func rotateEncryptionKeyEnabled(clusterLister v3.ClusterLister, clusterName string) bool {
	cluster, err := clusterLister.Get("", clusterName)
	if err != nil {
		return false
	}

	// check that encryption is enabled on cluster
	if cluster.Spec.RancherKubernetesEngineConfig == nil ||
		cluster.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig == nil ||
		!cluster.Spec.RancherKubernetesEngineConfig.Services.KubeAPI.SecretsEncryptionConfig.Enabled {
		return false
	}

	// Cluster should not be in updating
	return v32.ClusterConditionUpdated.IsTrue(cluster)
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

func hideClusterTemplateAnswers(answerMap map[string]interface{}, clusterSpecPwdFields map[string]interface{}) {
	for key := range answerMap {
		pwdVal := values.GetValueN(clusterSpecPwdFields, strings.Split(key, ".")...)
		if pwdVal != nil {
			//hide this answer
			delete(answerMap, key)
		}
	}
}

func (f *Formatter) CollectionFormatter(request *types.APIContext, collection *types.GenericCollection) {
	collection.AddAction(request, "createFromTemplate")
}

func gatherClusterSpecPwdFields(schemas *types.Schemas, schema *types.Schema) map[string]interface{} {

	data := map[string]interface{}{}

	for name, field := range schema.ResourceFields {
		fieldType := field.Type
		if strings.HasPrefix(fieldType, "array") {
			fieldType = strings.Split(fieldType, "[")[1]
			fieldType = fieldType[:len(fieldType)-1]
		}
		subSchema := schemas.Schema(&managementschema.Version, fieldType)
		if subSchema != nil {
			value := gatherClusterSpecPwdFields(schemas, subSchema)
			if len(value) > 0 {
				data[name] = value
			}
		} else {
			if field.Type == "password" {
				data[name] = "true"
			}
		}
	}

	return data
}
