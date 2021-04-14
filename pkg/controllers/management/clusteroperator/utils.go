package clusteroperator

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rancher/norman/condition"
	apiprojv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	utils2 "github.com/rancher/rancher/pkg/app"
	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	typesDialer "github.com/rancher/rancher/pkg/types/config/dialer"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"
)

const (
	localCluster = "local"
	systemNS     = "cattle-system"
)

type OperatorController struct {
	ClusterEnqueueAfter  func(name string, duration time.Duration)
	SecretsCache         wranglerv1.SecretCache
	TemplateCache        v3.CatalogTemplateCache
	ProjectCache         v3.ProjectCache
	AppLister            projectv3.AppLister
	AppClient            projectv3.AppInterface
	NsClient             corev1.NamespaceInterface
	ClusterClient        v3.ClusterClient
	CatalogManager       manager.CatalogManager
	SystemAccountManager *systemaccount.Manager
	DynamicClient        dynamic.NamespaceableResourceInterface
	ClientDialer         typesDialer.Factory
}

type operatorValues struct {
	HTTPProxy  string `json:"httpProxy,omitempty"`
	HTTPSProxy string `json:"httpsProxy,omitempty"`
	NoProxy    string `json:"noProxy,omitempty"`
}

func (e *OperatorController) DeployOperator(operator, operatorTemplate, shortName string) error {
	template, err := e.TemplateCache.Get(namespace.GlobalNamespace, operatorTemplate)
	if err != nil {
		return err
	}

	latestTemplateVersion, err := e.CatalogManager.LatestAvailableTemplateVersion(template, localCluster)
	if err != nil {
		return err
	}

	latestVersionID := latestTemplateVersion.ExternalID

	systemProject, err := project.GetSystemProject(localCluster, e.ProjectCache)
	if err != nil {
		return err
	}

	systemProjectID := ref.Ref(systemProject)
	_, systemProjectName := ref.Parse(systemProjectID)

	valuesYaml, err := generateValuesYaml()
	if err != nil {
		return err
	}

	app, err := e.AppLister.Get(systemProjectName, operator)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		logrus.Infof("deploying %s operator into local cluster's system project", shortName)
		creator, err := e.SystemAccountManager.GetSystemUser(localCluster)
		if err != nil {
			return err
		}

		appProjectName, err := utils2.EnsureAppProjectName(e.NsClient, systemProjectName, localCluster, systemNS, creator.Name)
		if err != nil {
			return err
		}

		desiredApp := &apiprojv3.App{
			ObjectMeta: v1.ObjectMeta{
				Name:      operator,
				Namespace: systemProjectName,
				Annotations: map[string]string{
					rbac.CreatorIDAnn: creator.Name,
				},
			},
			Spec: apiprojv3.AppSpec{
				Description:     fmt.Sprintf("Operator for provisioning %s clusters", shortName),
				ExternalID:      latestVersionID,
				ProjectName:     appProjectName,
				TargetNamespace: systemNS,
			},
		}

		desiredApp.Spec.ValuesYaml = valuesYaml

		if _, err = e.AppClient.Create(desiredApp); err != nil {
			return err
		}
	} else {
		if app.Spec.ExternalID == latestVersionID && app.Spec.ValuesYaml == valuesYaml {
			// app is up to date, no action needed
			return nil
		}
		logrus.Infof("updating %s operator in local cluster's system project", shortName)
		desiredApp := app.DeepCopy()
		desiredApp.Spec.ExternalID = latestVersionID
		desiredApp.Spec.ValuesYaml = valuesYaml
		// new version of operator upgrade available, update app
		if _, err = e.AppClient.Update(desiredApp); err != nil {
			return err
		}
	}

	return nil
}

func (e *OperatorController) SetUnknown(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsUnknown(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.Unknown(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.ClusterClient.Update(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, nil
}

func (e *OperatorController) SetTrue(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsTrue(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.True(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.ClusterClient.Update(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, nil
}

func (e *OperatorController) SetFalse(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsFalse(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.False(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.ClusterClient.Update(cluster)
	if err != nil {
		return cluster, err
	}
	return cluster, nil
}

// generateValuesYaml generates a YAML string containing any
// necessary values to override defaults in values.yaml. If
// no defaults need to be overwritten, an empty string will
// be returned.
func generateValuesYaml() (string, error) {
	values := operatorValues{
		HTTPProxy:  os.Getenv("HTTP_PROXY"),
		HTTPSProxy: os.Getenv("HTTPS_PROXY"),
		NoProxy:    os.Getenv("NO_PROXY"),
	}

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return "", err
	}

	return string(valuesYaml), nil
}

// RecordCAAndAPIEndpoint reads the cluster config's secret once available. The CA cert and API endpoint are then copied to the cluster status.
func (e *OperatorController) RecordCAAndAPIEndpoint(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	backoff := wait.Backoff{
		Duration: 2 * time.Second,
		Factor:   2,
		Jitter:   0,
		Steps:    6,
		Cap:      20 * time.Second,
	}

	var caSecret *corev1.Secret
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		caSecret, err = e.SecretsCache.Get(namespace.GlobalNamespace, cluster.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return false, err
			}
			logrus.Infof("waiting for cluster [%s] data needed to generate service account token", cluster.Name)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return cluster, fmt.Errorf("failed waiting for cluster [%s] secret: %s", cluster.Name, err)
	}

	apiEndpoint := string(caSecret.Data["endpoint"])
	if !strings.HasPrefix(apiEndpoint, "https://") {
		apiEndpoint = "https://" + apiEndpoint
	}
	caCert := string(caSecret.Data["ca"])
	if cluster.Status.APIEndpoint == apiEndpoint && cluster.Status.CACert == caCert {
		return cluster, nil
	}

	var currentCluster *mgmtv3.Cluster
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		currentCluster, err = e.ClusterClient.Get(cluster.Name, v1.GetOptions{})
		if err != nil {
			return err
		}
		currentCluster.Status.APIEndpoint = apiEndpoint
		currentCluster.Status.CACert = caCert
		currentCluster, err = e.ClusterClient.Update(currentCluster)
		return err
	})

	return currentCluster, err
}
