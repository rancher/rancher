package cluster

import (
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/norman/types"
	"github.com/rancher/norman/types/convert"
	"github.com/rancher/norman/types/values"
	gaccess "github.com/rancher/rancher/pkg/api/norman/customization/globalnamespaceaccess"
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
	clusterSpecPwdFields      map[string]interface{}
	SubjectAccessReviewClient v1.SubjectAccessReviewInterface
}

func NewFormatter(schemas *types.Schemas, managementContext *config.ScaledContext) *Formatter {
	clusterFormatter := Formatter{
		KontainerDriverLister:     managementContext.Management.KontainerDrivers("").Controller().Lister(),
		nodeLister:                managementContext.Management.Nodes("").Controller().Lister(),
		clusterSpecPwdFields:      gatherClusterSpecPwdFields(schemas, schemas.Schema(&managementschema.Version, client.ClusterSpecBaseType)),
		SubjectAccessReviewClient: managementContext.K8sClient.AuthorizationV1().SubjectAccessReviews(),
	}
	return &clusterFormatter
}

func canUserUpdateCluster(request *types.APIContext, resource *types.RawResource) bool {
	if request == nil || resource == nil {
		return false
	}
	cluster := map[string]interface{}{
		"id": resource.ID,
	}
	return request.AccessControl.CanDo(
		v3.ClusterGroupVersionKind.Group,
		v3.ClusterResource.Name,
		"update",
		request,
		cluster,
		resource.Schema) == nil
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
	if _, ok := resource.Values["rancherKubernetesEngineConfig"]; ok {
		resource.AddAction(request, v32.ClusterActionExportYaml)
		resource.AddAction(request, v32.ClusterActionRotateCertificates)
		if _, ok := values.GetValue(resource.Values, "rancherKubernetesEngineConfig", "services", "etcd", "backupConfig"); ok {
			resource.AddAction(request, v32.ClusterActionBackupEtcd)
			resource.AddAction(request, v32.ClusterActionRestoreFromEtcdBackup)
		}
		isActiveCluster := false
		if resource.Values["state"] == "active" {
			isActiveCluster = true
		}
		isWindowsCluster := false
		if resource.Values["windowsPreferedCluster"] == true {
			isWindowsCluster = true
		}
		if isActiveCluster && !isWindowsCluster {
			canUpdateCluster := canUserUpdateCluster(request, resource)
			logrus.Debugf("isActiveCluster: %v isWindowsCluster: %v user: %v, canUpdateCluster: %v", isActiveCluster, isWindowsCluster, request.Request.Header.Get("Impersonate-User"), canUpdateCluster)
			if canUpdateCluster {
				resource.AddAction(request, v32.ClusterActionRunSecurityScan)
			}
		}
	}

	if err := request.AccessControl.CanDo(v3.ClusterGroupVersionKind.Group, v3.ClusterResource.Name, "update", request, resource.Values, request.Schema); err == nil {
		if convert.ToBool(resource.Values["enableClusterMonitoring"]) {
			resource.AddAction(request, v32.ClusterActionDisableMonitoring)
			resource.AddAction(request, v32.ClusterActionEditMonitoring)
		} else {
			resource.AddAction(request, v32.ClusterActionEnableMonitoring)
		}
		if _, ok := resource.Values["rancherKubernetesEngineConfig"]; ok {
			if val, ok := values.GetValue(resource.Values, "clusterTemplateRevisionId"); ok && val == nil {
				callerID := request.Request.Header.Get(gaccess.ImpersonateUserHeader)
				if canCreateTemplates, _ := CanCreateRKETemplate(callerID, f.SubjectAccessReviewClient); canCreateTemplates {
					resource.AddAction(request, v32.ClusterActionSaveAsTemplate)
				}
			}
		}
	}

	if convert.ToBool(resource.Values["enableClusterMonitoring"]) {
		resource.AddAction(request, v32.ClusterActionViewMonitoring)
	}

	if gkeConfig, ok := resource.Values["googleKubernetesEngineConfig"]; ok {
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

	if eksConfig, ok := resource.Values["amazonElasticContainerServiceConfig"]; ok {
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
