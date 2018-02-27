package clusterpipeline

import (
	"github.com/pkg/errors"
	"github.com/rancher/rancher/pkg/controllers/user/pipeline/utils"
	"github.com/rancher/types/apis/apps/v1beta2"
	"github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	coretypev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbactypev1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
)

//Syncer is responsible for watching cluterpipeline config and enable/disable the pipeline.
//It creates cattle-pipeline namespace and deploys workloads in it when the pipeline is enabled.
//It removes cattle-pipeline namespace and cleans up related data when the pipeline is disabled.
type Syncer struct {
	namespaces          v1.NamespaceInterface
	secrets             v1.SecretInterface
	configmaps          coretypev1.ConfigMapInterface
	services            v1.ServiceInterface
	serviceAccounts     coretypev1.ServiceAccountInterface
	clusterRoleBindings rbactypev1.ClusterRoleBindingInterface
	deployments         v1beta2.DeploymentInterface

	clusterPipelines           v3.ClusterPipelineInterface
	pipelines                  v3.PipelineInterface
	pipelineLister             v3.PipelineLister
	pipelineExecutions         v3.PipelineExecutionInterface
	pipelineExecutionLister    v3.PipelineExecutionLister
	pipelineExecutionLogs      v3.PipelineExecutionLogInterface
	pipelineExecutionLogLister v3.PipelineExecutionLogLister
	sourceCodeCredentials      v3.SourceCodeCredentialInterface
	sourceCodeCredentialLister v3.SourceCodeCredentialLister
	sourceCodeRepositories     v3.SourceCodeRepositoryInterface
	sourceCodeRepositoryLister v3.SourceCodeRepositoryLister

	projectLister v3.ProjectLister
}

func Register(cluster *config.UserContext) {

	namespaces := cluster.Core.Namespaces("")
	secrets := cluster.Core.Secrets("")
	configmaps := cluster.K8sClient.CoreV1().ConfigMaps(utils.PipelineNamespace)
	services := cluster.Core.Services("")
	serviceAccounts := cluster.K8sClient.CoreV1().ServiceAccounts(utils.PipelineNamespace)
	clusterRoleBindings := cluster.K8sClient.RbacV1().ClusterRoleBindings()
	deployments := cluster.Apps.Deployments("")
	projectLister := cluster.Management.Management.Projects("").Controller().Lister()

	clusterPipelines := cluster.Management.Management.ClusterPipelines("")
	pipelines := cluster.Management.Management.Pipelines("")
	pipelineLister := pipelines.Controller().Lister()
	pipelineExecutions := cluster.Management.Management.PipelineExecutions("")
	pipelineExecutionLister := pipelineExecutions.Controller().Lister()
	pipelineExecutionLogs := cluster.Management.Management.PipelineExecutionLogs("")
	pipelineExecutionLogLister := pipelineExecutionLogs.Controller().Lister()
	sourceCodeCredentials := cluster.Management.Management.SourceCodeCredentials("")
	sourceCodeCredentialLister := sourceCodeCredentials.Controller().Lister()
	sourceCodeRepositories := cluster.Management.Management.SourceCodeRepositories("")
	sourceCodeRepositoryLister := sourceCodeRepositories.Controller().Lister()

	clusterPipelineSyncer := &Syncer{
		namespaces:          namespaces,
		secrets:             secrets,
		configmaps:          configmaps,
		services:            services,
		serviceAccounts:     serviceAccounts,
		clusterRoleBindings: clusterRoleBindings,
		deployments:         deployments,

		clusterPipelines:           clusterPipelines,
		pipelines:                  pipelines,
		pipelineLister:             pipelineLister,
		pipelineExecutions:         pipelineExecutions,
		pipelineExecutionLister:    pipelineExecutionLister,
		pipelineExecutionLogs:      pipelineExecutionLogs,
		pipelineExecutionLogLister: pipelineExecutionLogLister,
		sourceCodeCredentials:      sourceCodeCredentials,
		sourceCodeCredentialLister: sourceCodeCredentialLister,
		sourceCodeRepositories:     sourceCodeRepositories,
		sourceCodeRepositoryLister: sourceCodeRepositoryLister,

		projectLister: projectLister,
	}
	clusterPipelines.AddClusterScopedHandler("cluster-pipeline-syncer", cluster.ClusterName, clusterPipelineSyncer.Sync)
}

func (s *Syncer) Sync(key string, obj *v3.ClusterPipeline) error {
	if obj == nil || obj.Spec.ClusterName != obj.Name {
		return nil
	}
	//ensure clusterpipeline singleton in the cluster
	utils.InitClusterPipeline(s.clusterPipelines, obj.Spec.ClusterName)

	if obj.Spec.GithubConfig == nil {
		if err := s.cleanUp(obj.Spec.ClusterName); err != nil {
			return err
		}
	}

	if obj.Spec.Deploy {
		return s.deploy()
	}

	return s.destroy()
}

func (s *Syncer) destroy() error {
	if err := s.namespaces.Delete(utils.PipelineNamespace, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (s *Syncer) deploy() error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.PipelineNamespace,
		},
	}
	if _, err := s.namespaces.Create(ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create ns")
	}

	secret := getSecret()
	if _, err := s.secrets.Create(secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create secret")
	}

	configmap := getConfigMap()
	if _, err := s.configmaps.Create(configmap); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create configmap")
	}

	service := getJenkinsService()
	if _, err := s.services.Create(service); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create service")
	}

	agentservice := getJenkinsAgentService()
	if _, err := s.services.Create(agentservice); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create service")
	}

	sa := getServiceAccount()
	if _, err := s.serviceAccounts.Create(sa); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create service account")
	}
	rb := getRoleBindings()
	if _, err := s.clusterRoleBindings.Create(rb); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create role binding")
	}
	deployment := getJenkinsDeployment()
	if _, err := s.deployments.Create(deployment); err != nil && !apierrors.IsAlreadyExists(err) {
		return errors.Wrapf(err, "Error create deployment")
	}

	return nil
}

func (s *Syncer) cleanUp(clusterName string) error {

	//clean resource
	credentials, err := s.sourceCodeCredentialLister.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, credential := range credentials {
		if credential.Spec.ClusterName != clusterName {
			continue
		}
		if err := s.sourceCodeCredentials.DeleteNamespaced(credential.Namespace, credential.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	repositories, err := s.sourceCodeRepositoryLister.List("", labels.Everything())
	if err != nil {
		return err
	}
	for _, repo := range repositories {
		if repo.Spec.ClusterName != clusterName {
			continue
		}
		if err := s.sourceCodeRepositories.DeleteNamespaced(repo.Namespace, repo.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	projects, err := s.projectLister.List(clusterName, labels.Everything())
	if err != nil {
		return err
	}
	for _, project := range projects {
		if err := s.cleanUpInNamespace(project.Name); err != nil {
			return err
		}
	}

	return nil
}

func (s *Syncer) cleanUpInNamespace(ns string) error {
	pipelines, err := s.pipelineLister.List(ns, labels.Everything())
	if err != nil {
		return err
	}
	for _, pipeline := range pipelines {
		if err := s.pipelines.DeleteNamespaced(pipeline.Namespace, pipeline.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	executions, err := s.pipelineExecutionLister.List(ns, labels.Everything())
	if err != nil {
		return err
	}
	for _, execution := range executions {
		if err := s.pipelineExecutions.DeleteNamespaced(execution.Namespace, execution.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	logs, err := s.pipelineExecutionLogLister.List(ns, labels.Everything())
	if err != nil {
		return err
	}
	for _, log := range logs {
		if err := s.pipelineExecutionLogs.DeleteNamespaced(log.Namespace, log.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}
