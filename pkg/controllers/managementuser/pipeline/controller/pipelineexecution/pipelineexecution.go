package pipelineexecution

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	appsv1 "github.com/rancher/rancher/pkg/generated/norman/apps/v1"
	v1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	networkv1 "github.com/rancher/rancher/pkg/generated/norman/networking.k8s.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/notifiers"
	"github.com/rancher/rancher/pkg/pipeline/engine"
	"github.com/rancher/rancher/pkg/pipeline/utils"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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

type notifierRecipient struct {
	Notifier  *mv3.Notifier
	Recipient string
}

type executionSummary struct {
	Run                  int
	RepoName             string
	State                string
	CommitMessage        string
	Author               string
	GitRefURL            string
	PipelineExecutionURL string
	Event                string
	Duration             string
	Message              string
}

type Lifecycle struct {
	ctx                  context.Context
	systemAccountManager *systemaccount.Manager
	namespaceLister      v1.NamespaceLister
	namespaces           v1.NamespaceInterface
	secrets              v1.SecretInterface
	serviceLister        v1.ServiceLister
	managementSecrets    v1.SecretInterface
	services             v1.ServiceInterface
	serviceAccounts      v1.ServiceAccountInterface
	configMapLister      v1.ConfigMapLister
	configMaps           v1.ConfigMapInterface
	podLister            v1.PodLister
	pods                 v1.PodInterface
	networkPolicies      networkv1.NetworkPolicyInterface

	clusterRoleBindings rbacv1.ClusterRoleBindingInterface
	roleBindings        rbacv1.RoleBindingInterface
	deployments         appsv1.DeploymentInterface
	daemonsets          appsv1.DaemonSetInterface

	notifierLister             mv3.NotifierLister
	pipelineLister             v3.PipelineLister
	pipelines                  v3.PipelineInterface
	pipelineExecutionLister    v3.PipelineExecutionLister
	pipelineExecutions         v3.PipelineExecutionInterface
	pipelineSettingLister      v3.PipelineSettingLister
	pipelineEngine             engine.PipelineEngine
	sourceCodeCredentialLister v3.SourceCodeCredentialLister

	DialerFactory dialer.Factory
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
	notifierLister := cluster.Management.Management.Notifiers("").Controller().Lister()

	pipelineEngine := engine.New(cluster, true)
	pipelineExecutionLifecycle := &Lifecycle{
		ctx:                        ctx,
		systemAccountManager:       systemaccount.NewManager(cluster.Management),
		namespaces:                 namespaces,
		namespaceLister:            namespaceLister,
		secrets:                    secrets,
		managementSecrets:          managementSecrets,
		services:                   services,
		serviceLister:              serviceLister,
		serviceAccounts:            serviceAccounts,
		networkPolicies:            networkPolicies,
		clusterRoleBindings:        clusterRoleBindings,
		roleBindings:               roleBindings,
		deployments:                deployments,
		daemonsets:                 daemonsets,
		pods:                       pods,
		podLister:                  podLister,
		configMaps:                 configMaps,
		configMapLister:            configMapLister,
		pipelineLister:             pipelineLister,
		pipelines:                  pipelines,
		pipelineExecutionLister:    pipelineExecutionLister,
		pipelineExecutions:         pipelineExecutions,
		pipelineSettingLister:      pipelineSettingLister,
		pipelineEngine:             pipelineEngine,
		sourceCodeCredentialLister: sourceCodeCredentialLister,
		notifierLister:             notifierLister,

		DialerFactory: cluster.Management.Dialer,
	}
	stateSyncer := &ExecutionStateSyncer{
		clusterName:             clusterName,
		pipelineLister:          pipelineLister,
		pipelines:               pipelines,
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineExecutions:      pipelineExecutions,
		pipelineEngine:          pipelineEngine,
	}
	registryCertSyncer := &RegistryCertSyncer{
		clusterName:             clusterName,
		pods:                    pods,
		podLister:               podLister,
		secrets:                 secrets,
		secretLister:            secretLister,
		managementSecretLister:  managementSecretLister,
		namespaceLister:         namespaceLister,
		pipelineExecutionLister: pipelineExecutionLister,
		pipelineSettingLister:   pipelineSettingLister,
	}

	pipelineExecutions.AddClusterScopedLifecycle(ctx, pipelineExecutionLifecycle.GetName(), cluster.ClusterName, pipelineExecutionLifecycle)

	go stateSyncer.sync(ctx, syncStateInterval)
	go registryCertSyncer.sync(ctx, checkCertRotateInterval)

}

func (l *Lifecycle) Create(obj *v3.PipelineExecution) (runtime.Object, error) {
	return l.Sync(obj)
}

func (l *Lifecycle) Updated(obj *v3.PipelineExecution) (runtime.Object, error) {
	return l.Sync(obj)
}

func (l *Lifecycle) Sync(obj *v3.PipelineExecution) (runtime.Object, error) {
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
		return l.doFinish(obj)
	}

	//doIfRunning
	if v32.PipelineExecutionConditionInitialized.GetStatus(obj) != "" {
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
	} else if obj.Status.ExecutionState == utils.StateQueueing {
		obj.Status.ExecutionState = utils.StateWaiting
	}

	//doIfOnCreation
	if err := l.newExecutionUpdateLastRunState(obj); err != nil {
		return obj, err
	}
	v32.PipelineExecutionConditionInitialized.CreateUnknownIfNotExists(obj)
	obj.Labels[utils.PipelineFinishLabel] = "false"

	if err := l.deploy(obj.Spec.ProjectName); err != nil {
		obj.Labels[utils.PipelineFinishLabel] = "true"
		obj.Status.ExecutionState = utils.StateFailed
		v32.PipelineExecutionConditionInitialized.False(obj)
		v32.PipelineExecutionConditionInitialized.ReasonAndMessageFromError(obj, err)
	}

	if err := l.markLocalRegistryPort(obj); err != nil {
		return obj, err
	}

	return obj, nil
}

func (l *Lifecycle) doFinish(obj *v3.PipelineExecution) (*v3.PipelineExecution, error) {
	if err := l.doCleanup(obj); err != nil {
		return obj, err
	}

	shouldNotify, err := l.shouldNotify(obj)
	if err != nil {
		return obj, err
	}
	if shouldNotify {
		newObj, err := v32.PipelineExecutionConditionNotified.Once(obj, func() (runtime.Object, error) {
			return l.doNotify(obj)
		})
		if err != nil {
			return newObj.(*v3.PipelineExecution), err
		}
	}
	//start a queueing execution if there is any
	if err := l.startQueueingExecution(obj); err != nil {
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
	if v32.PipelineExecutionConditionInitialized.IsTrue(obj) {
		if err := l.pipelineEngine.StopExecution(obj); err != nil {
			return err
		}
		if _, err := l.pipelineEngine.SyncExecution(obj); err != nil {
			return err
		}
	}
	v32.PipelineExecutionConditionBuilt.Message(obj, "aborted by user")
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
	ns := utils.GetPipelineCommonName(obj.Spec.ProjectName)

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

func (l *Lifecycle) Remove(obj *v3.PipelineExecution) (runtime.Object, error) {
	if utils.IsFinishState(obj.Status.ExecutionState) {
		return obj, nil
	}
	return l.doFinish(obj)
}

func (l *Lifecycle) shouldNotify(obj *v3.PipelineExecution) (bool, error) {
	if obj.Spec.PipelineConfig.Notification != nil {
		if len(obj.Spec.PipelineConfig.Notification.Recipients) <= 0 {
			return false, nil
		}
		condition := obj.Spec.PipelineConfig.Notification.Condition
		if slice.ContainsString(condition, obj.Status.ExecutionState) {
			return true, nil
		} else if slice.ContainsString(condition, utils.ConditionChanged) {
			_, pipelineName := ref.Parse(obj.Spec.PipelineName)
			lastExecutionName := fmt.Sprintf("%s-%d", pipelineName, obj.Spec.Run-1)
			lastExecution, err := l.pipelineExecutionLister.Get(obj.Namespace, lastExecutionName)
			if apierrors.IsNotFound(err) {
				return false, nil
			} else if err != nil {
				return false, err
			}
			if utils.IsFinishState(lastExecution.Status.ExecutionState) &&
				lastExecution.Status.ExecutionState != obj.Status.ExecutionState {
				return true, nil
			}
		}
	}
	return false, nil
}

func (l *Lifecycle) doNotify(obj *v3.PipelineExecution) (runtime.Object, error) {
	toSendRecipients, err := l.getToSendRecipients(obj)
	if err != nil {
		return obj, err
	}
	message, err := defaultNotificationMessage(obj)
	if err != nil {
		return obj, err
	}
	clusterName, _ := ref.Parse(obj.Spec.ProjectName)
	clusterDialer, err := l.DialerFactory.ClusterDialer(clusterName)
	if err != nil {
		return nil, errors.Wrap(err, "error getting dialer")
	}
	if obj.Spec.PipelineConfig.Notification.Message != "" {
		message = obj.Spec.PipelineConfig.Notification.Message
	}
	var g errgroup.Group
	for i := range toSendRecipients {
		toSendRecipient := toSendRecipients[i]
		notifierMessage := &notifiers.Message{
			Content: message,
		}
		if toSendRecipient.Notifier.Spec.SMTPConfig != nil {
			repoName := getRepoNameFromURL(obj.Spec.RepositoryURL)
			notifierMessage.Title = fmt.Sprintf("Notification From Rancher: Pipeline #%d build for %s repo %s", obj.Spec.Run, repoName, obj.Status.ExecutionState)
			notifierMessage.Content = strings.Replace(message, "\n", "<br>\n", -1)
		}
		g.Go(func() error {
			return notifiers.SendMessage(l.ctx, toSendRecipient.Notifier, toSendRecipient.Recipient, notifierMessage, clusterDialer)
		})
	}
	return obj, g.Wait()
}

func (l *Lifecycle) getToSendRecipients(obj *v3.PipelineExecution) ([]notifierRecipient, error) {
	clusterName, _ := ref.Parse(obj.Spec.ProjectName)
	existingNotifiers, err := l.notifierLister.List(clusterName, labels.NewSelector())
	if err != nil {
		return nil, err
	}
	var toSendRecipients []notifierRecipient
	for _, recipient := range obj.Spec.PipelineConfig.Notification.Recipients {
		notifierName := recipient.Notifier
		for _, notifier := range existingNotifiers {
			_, name := ref.Parse(notifierName)
			if name == notifier.Spec.DisplayName || name == notifier.Name {
				toSendRecipients = append(toSendRecipients, struct {
					Notifier  *mv3.Notifier
					Recipient string
				}{Notifier: notifier, Recipient: recipient.Recipient})
			}
		}
	}
	return toSendRecipients, nil
}

func (l *Lifecycle) GetName() string {
	return "pipeline-execution-controller"
}

//reconcileRb grant access to pipeline service account inside project namespaces
func (l *Lifecycle) reconcileRb(projectName string) error {
	commonName := utils.GetPipelineCommonName(projectName)
	_, projectID := ref.Parse(projectName)

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

func defaultNotificationMessage(execution *v3.PipelineExecution) (string, error) {
	notificationTemplate := `
Pipeline execution #{{.Run}} for {{.RepoName}} repo ended in '{{.State}}' state
Commit message: {{.CommitMessage}}
Author: {{.Author}}
Git Ref URL: {{.GitRefURL}}
Pipeline execution URL: {{.PipelineExecutionURL}}
Event: {{.Event}}
Duration: {{.Duration}}
Message: {{.Message}}
`

	repoName := getRepoNameFromURL(execution.Spec.RepositoryURL)
	duration := "<Unknown>"
	endTime, err1 := time.Parse(time.RFC3339, execution.Status.Ended)
	startTime, err2 := time.Parse(time.RFC3339, execution.Status.Started)
	if err1 == nil && err2 == nil {
		duration = endTime.Sub(startTime).String()
	} else {
		logrus.Warnf("cannot parse duration of pipeline execution %s: %v,%v", execution.Name, err1, err2)
	}
	buildLink := fmt.Sprintf("%s/p/%s/pipeline/pipelines/%s/run/%d",
		settings.ServerURL.Get(),
		execution.Spec.ProjectName,
		execution.Spec.PipelineName,
		execution.Spec.Run,
	)
	builtMessage := "Success"
	if v32.PipelineExecutionConditionBuilt.IsFalse(execution) {
		builtMessage = v32.PipelineExecutionConditionBuilt.GetMessage(execution)
	}
	buf := &bytes.Buffer{}
	data := executionSummary{
		Run:                  execution.Spec.Run,
		RepoName:             repoName,
		State:                execution.Status.ExecutionState,
		CommitMessage:        execution.Spec.Message,
		Author:               execution.Spec.Author,
		GitRefURL:            execution.Spec.HTMLLink,
		PipelineExecutionURL: buildLink,
		Event:                execution.Spec.Event,
		Duration:             duration,
		Message:              builtMessage,
	}
	t, err := template.New("notification").Parse(notificationTemplate)
	if err != nil {
		return "", err
	}
	if err := t.Execute(buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func getRepoNameFromURL(repoURL string) string {
	reg := regexp.MustCompile(".*/([^/]*?)/([^/]*?).git")
	match := reg.FindStringSubmatch(repoURL)
	if len(match) != 3 {
		logrus.Warnf("failed to parse git repo url: %s", repoURL)
		return fmt.Sprintf("<%s>", repoURL)
	}
	return fmt.Sprintf("%s/%s", match[1], match[2])
}
