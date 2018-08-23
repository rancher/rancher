package pipelineexecution

import (
	"context"
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/pipeline/engine"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	networkv1 "github.com/rancher/types/apis/networking.k8s.io/v1"
	"github.com/rancher/types/apis/project.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strconv"
	"strings"
)

//This controller is responsible for
// a) setup necessary components when pipeline executions are triggered
// b) maintain the execution queue
// c) terminate an execution when it is aborted

const (
	projectIDLabel   = "field.cattle.io/projectId"
	roleCreateNs     = "create-ns"
	roleEditNsSuffix = "-namespaces-edit"
	roleAdmin        = "admin"
)

type Lifecycle struct {
	namespaceLister   v1.NamespaceLister
	namespaces        v1.NamespaceInterface
	secrets           v1.SecretInterface
	serviceLister     v1.ServiceLister
	managementSecrets v1.SecretInterface
	services          v1.ServiceInterface
	serviceAccounts   v1.ServiceAccountInterface
	configMapLister   v1.ConfigMapLister
	configMaps        v1.ConfigMapInterface
	podLister         v1.PodLister
	pods              v1.PodInterface
	networkPolicies   networkv1.NetworkPolicyInterface

	clusterRoleBindings rbacv1.ClusterRoleBindingInterface
	roleBindings        rbacv1.RoleBindingInterface
	deployments         v1beta2.DeploymentInterface
	daemonsets          v1beta2.DaemonSetInterface

	pipelineLister             v3.PipelineLister
	pipelines                  v3.PipelineInterface
	pipelineExecutionLister    v3.PipelineExecutionLister
	pipelineExecutions         v3.PipelineExecutionInterface
	pipelineSettingLister      v3.PipelineSettingLister
	pipelineEngine             engine.PipelineEngine
	sourceCodeCredentialLister v3.SourceCodeCredentialLister
}

func Register(ctx context.Context, cluster *config.UserContext) {
	clusterName := cluster.ClusterName

	namespaces := cluster.Core.Namespaces("")
	namespaceLister := cluster.Core.Namespaces("").Controller().Lister()
	secrets := cluster.Core.Secrets("")
	secretLister := secrets.Controller().Lister()
	managementSecrets := cluster.Management.Core.Secrets("")
	managementSecretLister := managementSecrets.Controller().Lister()
	services := cluster.Core.Services("")
	serviceLister := services.Controller().Lister()
	serviceAccounts := cluster.Core.ServiceAccounts("")
	networkPolicies := cluster.Networking.NetworkPolicies("")
	clusterRoleBindings := cluster.RBAC.ClusterRoleBindings("")
	roleBindings := cluster.RBAC.RoleBindings("")
	deployments := cluster.Apps.Deployments("")
	daemonsets := cluster.Apps.DaemonSets("")
	pods := cluster.Core.Pods("")
	podLister := pods.Controller().Lister()
	configMaps := cluster.Core.ConfigMaps("")
	configMapLister := configMaps.Controller().Lister()

	pipelines := cluster.Management.Project.Pipelines("")
	pipelineLister := pipelines.Controller().Lister()
	pipelineExecutions := cluster.Management.Project.PipelineExecutions("")
	pipelineExecutionLister := pipelineExecutions.Controller().Lister()
	pipelineSettingLister := cluster.Management.Project.PipelineSettings("").Controller().Lister()
	sourceCodeCredentialLister := cluster.Management.Project.SourceCodeCredentials("").Controller().Lister()

	pipelineEngine := engine.New(cluster)
	pipelineExecutionLifecycle := &Lifecycle{
		namespaces:          namespaces,
		namespaceLister:     namespaceLister,
		secrets:             secrets,
		managementSecrets:   managementSecrets,
		services:            services,
		serviceLister:       serviceLister,
		serviceAccounts:     serviceAccounts,
		networkPolicies:     networkPolicies,
		clusterRoleBindings: clusterRoleBindings,
		roleBindings:        roleBindings,
		deployments:         deployments,
		daemonsets:          daemonsets,
		pods:                pods,
		podLister:           podLister,
		configMaps:          configMaps,
		configMapLister:     configMapLister,

		pipelineLister:             pipelineLister,
		pipelines:                  pipelines,
		pipelineExecutionLister:    pipelineExecutionLister,
		pipelineExecutions:         pipelineExecutions,
		pipelineSettingLister:      pipelineSettingLister,
		pipelineEngine:             pipelineEngine,
		sourceCodeCredentialLister: sourceCodeCredentialLister,
	}
	stateSyncer := &ExecutionStateSyncer{
		clusterName: clusterName,

		pipelineLister:          pipelineLister,
		pipelines:               pipelines,
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineExecutions:      pipelineExecutions,
		pipelineEngine:          pipelineEngine,
	}
	registryCertSyncer := &RegistryCertSyncer{
		clusterName: clusterName,

		pods:                    pods,
		podLister:               podLister,
		secrets:                 secrets,
		secretLister:            secretLister,
		managementSecretLister:  managementSecretLister,
		namespaceLister:         namespaceLister,
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineSettingLister:   pipelineSettingLister,
	}

	pipelineExecutions.AddClusterScopedLifecycle(pipelineExecutionLifecycle.GetName(), cluster.ClusterName, pipelineExecutionLifecycle)

	go stateSyncer.sync(ctx, syncStateInterval)
	go registryCertSyncer.sync(ctx, checkCertRotateInterval)

}

func (l *Lifecycle) Create(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return l.Sync(obj)
}

func (l *Lifecycle) Updated(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return l.Sync(obj)
}

func (l *Lifecycle) Sync(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	if obj == nil || obj.DeletionTimestamp != nil {
		return obj, nil
	}

	//doIfAbort
	if obj.Status.ExecutionState == utils.StateAborted {
		if err := l.doStop(obj); err != nil {
			return obj, err
		}
	}

	//doIfFinish
	if obj.Labels != nil && obj.Labels[utils.PipelineFinishLabel] == "true" {
		if err := l.doCleanup(obj); err != nil {
			return obj, err
		}
		//start a queueing execution if there is any
		if err := l.startQueueingExecution(obj); err != nil {
			return obj, err
		}
		return obj, nil
	}

	//doIfRunning
	if v3.PipelineExecutionConditionInitialized.GetStatus(obj) != "" {
		return obj, nil
	}

	//doIfExceedQuota
	exceed, err := l.exceedQuota(obj)
	if err != nil {
		return obj, err
	}
	if exceed {
		obj.Status.ExecutionState = utils.StateQueueing
		obj.Labels[utils.PipelineFinishLabel] = ""

		if err := l.newExecutionUpdateLastRunState(obj); err != nil {
			return obj, err
		}

		return obj, nil
	}

	//doIfOnCreation
	if err := l.newExecutionUpdateLastRunState(obj); err != nil {
		return obj, err
	}
	v3.PipelineExecutionConditionInitialized.CreateUnknownIfNotExists(obj)
	obj.Labels[utils.PipelineFinishLabel] = "false"

	if err := l.deploy(obj); err != nil {
		obj.Labels[utils.PipelineFinishLabel] = "true"
		obj.Status.ExecutionState = utils.StateFailed
		v3.PipelineExecutionConditionInitialized.False(obj)
		v3.PipelineExecutionConditionInitialized.ReasonAndMessageFromError(obj, err)
	}

	if err := l.markLocalRegistryPort(obj); err != nil {
		return obj, err
	}

	return obj, nil
}

func (l *Lifecycle) markLocalRegistryPort(obj *v3.PipelineExecution) error {
	cm, err := l.configMaps.GetNamespaced(utils.PipelineNamespace, utils.ProxyConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	portMap, err := utils.GetRegistryPortMapping(cm)
	if err != nil {
		return err
	}
	curPort := ""
	if obj.Annotations != nil && obj.Annotations[utils.LocalRegistryPortLabel] != "" {
		curPort = obj.Annotations[utils.LocalRegistryPortLabel]
	}
	_, projectID := ref.Parse(obj.Spec.ProjectName)
	port := portMap[projectID]
	if port != curPort {
		toUpdate := obj.DeepCopy()
		if toUpdate.Annotations == nil {
			toUpdate.Annotations = map[string]string{}
		}
		toUpdate.Annotations[utils.LocalRegistryPortLabel] = port
		if _, err := l.pipelineExecutions.Update(toUpdate); err != nil {
			return err
		}
	}
	return nil
}

func (l *Lifecycle) newExecutionUpdateLastRunState(obj *v3.PipelineExecution) error {
	ns, name := ref.Parse(obj.Spec.PipelineName)
	pipeline, err := l.pipelineLister.Get(ns, name)
	if err != nil {
		return err
	}
	if obj.Spec.Run == pipeline.Status.NextRun {
		pipeline.Status.NextRun++
		pipeline.Status.LastExecutionID = ref.Ref(obj)
		pipeline.Status.LastStarted = obj.Status.Started
	}
	if pipeline.Status.LastExecutionID == ref.Ref(obj) {
		pipeline.Status.LastRunState = obj.Status.ExecutionState
	}
	_, err = l.pipelines.Update(pipeline)
	return err
}
func (l *Lifecycle) startQueueingExecution(obj *v3.PipelineExecution) error {
	_, projectID := ref.Parse(obj.Spec.ProjectName)
	set := labels.Set(map[string]string{utils.PipelineFinishLabel: ""})
	queueingExecutions, err := l.pipelineExecutionLister.List(projectID, set.AsSelector())
	if err != nil {
		return err
	}
	if len(queueingExecutions) == 0 {
		return nil
	}
	oldestTime := queueingExecutions[0].CreationTimestamp
	toRunExecution := queueingExecutions[0]
	for _, e := range queueingExecutions {
		if e.CreationTimestamp.Before(&oldestTime) {
			oldestTime = e.CreationTimestamp
			toRunExecution = e
		}
	}
	toRunExecution = toRunExecution.DeepCopy()
	toRunExecution.Status.ExecutionState = utils.StateWaiting
	_, err = l.pipelineExecutions.Update(toRunExecution)
	return err
}

func (l *Lifecycle) exceedQuota(obj *v3.PipelineExecution) (bool, error) {
	_, projectID := ref.Parse(obj.Spec.ProjectName)
	quotaSetting, err := l.pipelineSettingLister.Get(projectID, utils.SettingExecutorQuota)
	if apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	quotaStr := quotaSetting.Default
	if quotaSetting.Value != "" {
		quotaStr = quotaSetting.Value
	}
	quota, err := strconv.Atoi(quotaStr)
	if err != nil || quota <= 0 {
		return false, nil
	}
	set := labels.Set(map[string]string{utils.PipelineFinishLabel: "false"})
	runningExecutions, err := l.pipelineExecutionLister.List(projectID, set.AsSelector())
	if err != nil {
		return false, err
	}
	if len(runningExecutions) >= quota {
		return true, nil
	}
	return false, nil
}

func (l *Lifecycle) doStop(obj *v3.PipelineExecution) error {
	if v3.PipelineExecutionConditionInitialized.IsTrue(obj) {
		if err := l.pipelineEngine.StopExecution(obj); err != nil {
			return err
		}
		if _, err := l.pipelineEngine.SyncExecution(obj); err != nil {
			return err
		}
	}
	v3.PipelineExecutionConditionBuilt.Message(obj, "aborted by user")
	for i := range obj.Status.Stages {
		if obj.Status.Stages[i].State == utils.StateBuilding {
			obj.Status.Stages[i].State = utils.StateAborted
		}
		for j := range obj.Status.Stages[i].Steps {
			if obj.Status.Stages[i].Steps[j].State == utils.StateBuilding {
				obj.Status.Stages[i].Steps[j].State = utils.StateAborted
			}
		}
	}

	//check and update lastrunstate of the pipeline when necessary
	ns, name := ref.Parse(obj.Spec.PipelineName)
	p, err := l.pipelineLister.Get(ns, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if p != nil && p.Status.LastExecutionID == obj.Namespace+":"+obj.Name &&
		p.Status.LastRunState != obj.Status.ExecutionState {
		p.Status.LastRunState = obj.Status.ExecutionState
		if _, err := l.pipelines.Update(p); err != nil {
			return err
		}
	}

	return nil
}

func (l *Lifecycle) doCleanup(obj *v3.PipelineExecution) error {
	//Clean up on exception cases
	if err := l.pipelineEngine.StopExecution(obj); err != nil {
		return err
	}
	ns := utils.GetPipelineCommonName(obj)

	labelSet := labels.Set{
		utils.LabelKeyApp:       utils.JenkinsName,
		utils.LabelKeyExecution: obj.Name,
	}
	slavePods, err := l.podLister.List(ns, labelSet.AsSelector())
	if err != nil {
		return err
	}
	for _, pod := range slavePods {
		if err := l.pods.DeleteNamespaced(ns, pod.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
			return err
		}
	}
	return nil
}

func (l *Lifecycle) Remove(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	return obj, nil
}

func (l *Lifecycle) GetName() string {
	return "pipeline-execution-controller"
}

//reconcileRb grant access to pipeline service account inside project namespaces
func (l *Lifecycle) reconcileRb(obj *v3.PipelineExecution) error {
	commonName := utils.GetPipelineCommonName(obj)
	_, projectID := ref.Parse(obj.Spec.ProjectName)

	namespaces, err := l.namespaceLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "Error list cluster namespaces")
	}
	var namespacesInProject []*corev1.Namespace
	for _, namespace := range namespaces {
		parts := strings.Split(namespace.Annotations[projectIDLabel], ":")
		if len(parts) == 2 && parts[1] == projectID {
			namespacesInProject = append(namespacesInProject, namespace)
		} else {
			if err := l.roleBindings.DeleteNamespaced(namespace.Name, commonName, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
	}
	for _, namespace := range namespacesInProject {
		rb := getRoleBindings(namespace.Name, commonName)
		if _, err := l.roleBindings.Create(rb); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "Error create role binding")
		}
	}

	clusterRbs := []string{roleCreateNs, projectID + roleEditNsSuffix}
	for _, crbName := range clusterRbs {
		crb := getClusterRoleBindings(commonName, crbName)
		if _, err := l.clusterRoleBindings.Create(crb); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "Error create cluster role binding")
		}
	}

	return nil
}
