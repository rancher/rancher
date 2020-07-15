package configsyncer

import (
	"fmt"
	"sort"
	"strings"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	"github.com/rancher/norman/controller"
	loggingconfig "github.com/rancher/rancher/pkg/controllers/user/logging/config"
	"github.com/rancher/rancher/pkg/controllers/user/logging/passwordgetter"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/types/config"

	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	passwordSecretPrefix = "cattle-global-data:"
)

// This controller is responsible for generate fluentd config
// and updating the config secret
// so the config reload could detect the file change and reload

func NewConfigSyncer(cluster *config.UserContext, SecretManager *SecretManager) *ConfigSyncer {
	clusterName := cluster.ClusterName
	clusterLoggingLister := cluster.Management.Management.ClusterLoggings(clusterName).Controller().Lister()
	projectLoggingLister := cluster.Management.Management.ProjectLoggings(metav1.NamespaceAll).Controller().Lister()
	namespaceLister := cluster.Core.Namespaces(metav1.NamespaceAll).Controller().Lister()
	secrets := cluster.Management.Core.Secrets(metav1.NamespaceAll)

	configGenerator := NewConfigGenerator(clusterName, projectLoggingLister, namespaceLister)
	passwordGetter := passwordgetter.NewPasswordGetter(secrets)

	return &ConfigSyncer{
		apps:                 cluster.Management.Project.Apps(metav1.NamespaceAll),
		appLister:            cluster.Management.Project.Apps(metav1.NamespaceAll).Controller().Lister(),
		clusterName:          clusterName,
		clusterLoggingLister: clusterLoggingLister,
		projectLoggingLister: projectLoggingLister,
		projectLister:        cluster.Management.Management.Projects(clusterName).Controller().Lister(),
		secretManager:        SecretManager,
		configGenerator:      configGenerator,
		passwordGetter:       passwordGetter,
	}
}

type ConfigSyncer struct {
	apps                 projectv3.AppInterface
	appLister            projectv3.AppLister
	clusterName          string
	clusterLoggingLister mgmtv3.ClusterLoggingLister
	projectLoggingLister mgmtv3.ProjectLoggingLister
	projectLister        mgmtv3.ProjectLister
	secretManager        *SecretManager
	configGenerator      *ConfigGenerator
	passwordGetter       *passwordgetter.PasswordGetter
}

func (s *ConfigSyncer) NamespaceSync(key string, obj *k8scorev1.Namespace) (runtime.Object, error) {
	return obj, s.sync()
}

func (s *ConfigSyncer) ClusterLoggingSync(key string, obj *mgmtv3.ClusterLogging) (runtime.Object, error) {
	return obj, s.sync()
}

func (s *ConfigSyncer) ProjectLoggingSync(key string, obj *mgmtv3.ProjectLogging) (runtime.Object, error) {
	return obj, s.sync()
}

func (s *ConfigSyncer) sync() error {
	project, err := project.GetSystemProject(s.clusterName, s.projectLister)
	if err != nil {
		return err
	}

	systemProjectName := project.Name
	systemProjectID := fmt.Sprintf("%s:%s", project.Namespace, project.Name)

	isDeployed, err := s.isAppDeploy(systemProjectName)
	if err != nil {
		return err
	}

	if !isDeployed {
		return nil
	}

	allClusterLoggings, err := s.clusterLoggingLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List cluster loggings failed")
	}

	var clusterLoggings []*mgmtv3.ClusterLogging
	for _, logging := range allClusterLoggings {
		cp := logging.DeepCopy()
		if err := s.passwordGetter.GetPasswordFromSecret(&cp.Spec.LoggingTargets); err != nil {
			return errors.Wrap(err, "get password from secret failed")
		}
		clusterLoggings = append(clusterLoggings, cp)
	}

	allProjectLoggings, err := s.projectLoggingLister.List("", labels.NewSelector())
	if err != nil {
		return errors.Wrapf(err, "List project logging failed")
	}

	var projectLoggings []*mgmtv3.ProjectLogging
	for _, logging := range allProjectLoggings {
		if controller.ObjectInCluster(s.clusterName, logging) {
			cp := logging.DeepCopy()
			if err := s.passwordGetter.GetPasswordFromSecret(&cp.Spec.LoggingTargets); err != nil {
				return errors.Wrap(err, "get password from secret failed")
			}

			projectLoggings = append(projectLoggings, cp)
		}
	}

	sort.Slice(projectLoggings, func(i, j int) bool {
		return projectLoggings[i].Name < projectLoggings[j].Name
	})

	if err = s.syncSSLCert(clusterLoggings, projectLoggings); err != nil {
		return err
	}

	if err = s.syncClusterConfig(clusterLoggings, systemProjectID); err != nil {
		return err
	}

	return s.syncProjectConfig(projectLoggings, systemProjectID)
}

func (s *ConfigSyncer) isAppDeploy(appNamespace string) (bool, error) {
	appName := loggingconfig.AppName
	app, err := s.appLister.Get(appNamespace, appName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, errors.Wrapf(err, "get app %s failed", appName)
	}

	if app.DeletionTimestamp != nil {
		return false, nil
	}

	return true, nil
}

func (s *ConfigSyncer) syncClusterConfig(clusterLoggings []*mgmtv3.ClusterLogging, systemProjectID string) error {
	secretName := loggingconfig.RancherLoggingConfigSecretName()
	namespace := loggingconfig.LoggingNamespace

	var clusterLogging *mgmtv3.ClusterLogging
	if len(clusterLoggings) != 0 {
		clusterLogging = clusterLoggings[0]
	}

	buf, err := s.configGenerator.GenerateClusterLoggingConfig(clusterLogging, systemProjectID, loggingconfig.DefaultCertDir)
	if err != nil {
		return err
	}

	data := map[string][]byte{
		loggingconfig.LoggingSecretClusterConfigKey: buf,
	}

	return s.secretManager.updateSecret(secretName, namespace, data)
}

func (s *ConfigSyncer) syncProjectConfig(projectLoggings []*mgmtv3.ProjectLogging, systemProjectID string) error {
	secretName := loggingconfig.RancherLoggingConfigSecretName()
	namespace := loggingconfig.LoggingNamespace

	buf, err := s.configGenerator.GenerateProjectLoggingConfig(projectLoggings, systemProjectID, loggingconfig.DefaultCertDir)
	if err != nil {
		return err
	}

	data := map[string][]byte{
		loggingconfig.LoggingSecretProjectConfigKey: buf,
	}

	return s.secretManager.updateSecret(secretName, namespace, data)
}

func (s *ConfigSyncer) syncSSLCert(clusterLoggings []*mgmtv3.ClusterLogging, projectLoggings []*mgmtv3.ProjectLogging) error {
	secretname := loggingconfig.RancherLoggingSSLSecretName()
	namespace := loggingconfig.LoggingNamespace

	sslConfig := make(map[string][]byte)
	for _, v := range clusterLoggings {
		ca, cert, key := GetSSLConfig(v.Spec.LoggingTargets)
		sslConfig[loggingconfig.SecretDataKeyCa(loggingconfig.ClusterLevel, v.Namespace)] = []byte(ca)
		sslConfig[loggingconfig.SecretDataKeyCert(loggingconfig.ClusterLevel, v.Namespace)] = []byte(cert)
		sslConfig[loggingconfig.SecretDataKeyCertKey(loggingconfig.ClusterLevel, v.Namespace)] = []byte(key)
	}

	for _, v := range projectLoggings {
		target := v.Spec.LoggingTargets
		ca, cert, key := GetSSLConfig(target)
		projectKey := strings.Replace(v.Spec.ProjectName, ":", "_", -1)
		caByte := []byte(ca)
		sslConfig[loggingconfig.SecretDataKeyCa(loggingconfig.ProjectLevel, projectKey)] = caByte
		sslConfig[loggingconfig.SecretDataKeyCert(loggingconfig.ProjectLevel, projectKey)] = []byte(cert)
		sslConfig[loggingconfig.SecretDataKeyCertKey(loggingconfig.ProjectLevel, projectKey)] = []byte(key)
	}

	return s.secretManager.updateSecret(secretname, namespace, sslConfig)
}

func GetSSLConfig(target v32.LoggingTargets) (string, string, string) {
	var certificate, clientCert, clientKey string
	if target.ElasticsearchConfig != nil {
		certificate = target.ElasticsearchConfig.Certificate
		clientCert = target.ElasticsearchConfig.ClientCert
		clientKey = target.ElasticsearchConfig.ClientKey
	} else if target.SplunkConfig != nil {
		certificate = target.SplunkConfig.Certificate
		clientCert = target.SplunkConfig.ClientCert
		clientKey = target.SplunkConfig.ClientKey
	} else if target.KafkaConfig != nil {
		certificate = target.KafkaConfig.Certificate
		clientCert = target.KafkaConfig.ClientCert
		clientKey = target.KafkaConfig.ClientKey
	} else if target.SyslogConfig != nil {
		certificate = target.SyslogConfig.Certificate
		clientCert = target.SyslogConfig.ClientCert
		clientKey = target.SyslogConfig.ClientKey
	} else if target.FluentForwarderConfig != nil {
		certificate = target.FluentForwarderConfig.Certificate
		clientCert = target.FluentForwarderConfig.ClientCert
		clientKey = target.FluentForwarderConfig.ClientKey
	} else if target.CustomTargetConfig != nil {
		certificate = target.CustomTargetConfig.Certificate
		clientCert = target.CustomTargetConfig.ClientCert
		clientKey = target.CustomTargetConfig.ClientKey
	}

	return certificate, clientCert, clientKey
}
