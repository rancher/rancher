package gke

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/gke-operator/controller"
	gkev1 "github.com/rancher/gke-operator/pkg/apis/gke.cattle.io/v1"
	"github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apiprojv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	utils2 "github.com/rancher/rancher/pkg/app"
	"github.com/rancher/rancher/pkg/catalog/manager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterupstreamrefresher"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/dialer"
	v3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	projectv3 "github.com/rancher/rancher/pkg/generated/norman/project.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/util"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/project"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	typesDialer "github.com/rancher/rancher/pkg/types/config/dialer"
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"
)

const (
	systemNS            = "cattle-system"
	gkeAPIGroup         = "gke.cattle.io"
	gkeV1               = "gke.cattle.io/v1"
	gkeOperatorTemplate = "system-library-rancher-gke-operator"
	gkeOperator         = "rancher-gke-operator"
	localCluster        = "local"
	enqueueTime         = time.Second * 5
)

type gkeOperatorValues struct {
	HTTPProxy  string `json:"httpProxy,omitempty"`
	HTTPSProxy string `json:"httpsProxy,omitempty"`
	NoProxy    string `json:"noProxy,omitempty"`
}

type gkeOperatorController struct {
	clusterEnqueueAfter  func(name string, duration time.Duration)
	secretsCache         wranglerv1.SecretCache
	templateCache        v3.CatalogTemplateCache
	projectCache         v3.ProjectCache
	appLister            projectv3.AppLister
	appClient            projectv3.AppInterface
	nsClient             corev1.NamespaceInterface
	clusterClient        v3.ClusterClient
	catalogManager       manager.CatalogManager
	systemAccountManager *systemaccount.Manager
	dynamicClient        dynamic.NamespaceableResourceInterface
	clientDialer         typesDialer.Factory
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext) {
	gkeClusterConfigResource := schema.GroupVersionResource{
		Group:    gkeAPIGroup,
		Version:  "v1",
		Resource: "gkeclusterconfigs",
	}

	gkeCCDynamicClient := mgmtCtx.DynamicClient.Resource(gkeClusterConfigResource)
	e := &gkeOperatorController{
		clusterEnqueueAfter:  wContext.Mgmt.Cluster().EnqueueAfter,
		secretsCache:         wContext.Core.Secret().Cache(),
		templateCache:        wContext.Mgmt.CatalogTemplate().Cache(),
		projectCache:         wContext.Mgmt.Project().Cache(),
		appLister:            mgmtCtx.Project.Apps("").Controller().Lister(),
		appClient:            mgmtCtx.Project.Apps(""),
		nsClient:             mgmtCtx.Core.Namespaces(""),
		clusterClient:        wContext.Mgmt.Cluster(),
		catalogManager:       mgmtCtx.CatalogManager,
		systemAccountManager: systemaccount.NewManager(mgmtCtx),
		dynamicClient:        gkeCCDynamicClient,
		clientDialer:         mgmtCtx.Dialer,
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "gke-operator-controller", e.onClusterChange)
}

func (e *gkeOperatorController) onClusterChange(key string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Spec.GKEConfig == nil {
		return cluster, nil
	}

	if err := e.deployGKEOperator(); err != nil {
		failedToDeployGKEOperatorErr := "failed to deploy gke-operator: %v"
		var conditionErr error
		if cluster.Spec.GKEConfig.Imported {
			cluster, conditionErr = e.setFalse(cluster, apimgmtv3.ClusterConditionPending, fmt.Sprintf(failedToDeployGKEOperatorErr, err))
			if conditionErr != nil {
				return cluster, conditionErr
			}
		} else {
			cluster, conditionErr = e.setFalse(cluster, apimgmtv3.ClusterConditionProvisioned, fmt.Sprintf(failedToDeployGKEOperatorErr, err))
			if conditionErr != nil {
				return cluster, conditionErr
			}
		}
		return cluster, err
	}

	// set driver name
	if cluster.Status.Driver == "" {
		cluster = cluster.DeepCopy()
		cluster.Status.Driver = apimgmtv3.ClusterDriverGKE
		var err error
		cluster, err = e.clusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	// get gke Cluster Config, if it does not exist, create it
	gkeClusterConfigDynamic, err := e.dynamicClient.Namespace(namespace.GlobalNamespace).Get(context.TODO(), cluster.Name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return cluster, err
		}

		cluster, err = e.setUnknown(cluster, apimgmtv3.ClusterConditionWaiting, "Waiting for API to be available")
		if err != nil {
			return cluster, err
		}

		gkeClusterConfigDynamic, err = buildGKECCCreateObject(cluster)
		if err != nil {
			return cluster, err
		}

		gkeClusterConfigDynamic, err = e.dynamicClient.Namespace(namespace.GlobalNamespace).Create(context.TODO(), gkeClusterConfigDynamic, v1.CreateOptions{})
		if err != nil {
			return cluster, err
		}

	}

	gkeClusterConfigMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cluster.Spec.GKEConfig)
	if err != nil {
		return cluster, err
	}

	// check for changes between gke spec on cluster and the gke spec on the gkeClusterConfig object
	if !reflect.DeepEqual(gkeClusterConfigMap, gkeClusterConfigDynamic.Object["spec"]) {
		logrus.Infof("change detected for cluster [%s], updating GKEClusterConfig", cluster.Name)
		return e.updateGKEClusterConfig(cluster, gkeClusterConfigDynamic, gkeClusterConfigMap)
	}

	// get gke Cluster Config's phase
	status, _ := gkeClusterConfigDynamic.Object["status"].(map[string]interface{})
	phase, _ := status["phase"]
	failureMessage, _ := status["failureMessage"].(string)
	if strings.Contains(failureMessage, "403") {
		failureMessage = fmt.Sprintf("cannot access gke, check cloud credential: %s", failureMessage)
	}

	switch phase {
	case "creating":
		// set provisioning to unknown
		cluster, err = e.setUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		if cluster.Status.GKEStatus.UpstreamSpec == nil {
			cluster, err = e.setInitialUpstreamSpec(cluster)
			if err != nil {
				return cluster, err
			}
			return cluster, nil
		}

		e.clusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			logrus.Infof("waiting for cluster GKE [%s] to finish creating", cluster.Name)
			return e.setUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		}
		logrus.Infof("waiting for cluster GKE [%s] create failure to be resolved", cluster.Name)
		return e.setFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	case "active":
		if cluster.Spec.GKEConfig.Imported {
			if cluster.Status.GKEStatus.UpstreamSpec == nil {
				// non imported clusters will have already had upstream spec set
				return e.setInitialUpstreamSpec(cluster)
			}

			if apimgmtv3.ClusterConditionPending.IsUnknown(cluster) {
				cluster = cluster.DeepCopy()
				apimgmtv3.ClusterConditionPending.True(cluster)
				cluster, err = e.clusterClient.Update(cluster)
				if err != nil {
					return cluster, err
				}
			}
		}

		cluster, err = e.setTrue(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		if cluster.Status.APIEndpoint == "" {
			return e.recordCAAndAPIEndpoint(cluster)
		}

		if cluster.Status.GKEStatus.PrivateRequiresTunnel == nil &&
			cluster.Status.GKEStatus.UpstreamSpec.PrivateClusterConfig != nil &&
			cluster.Status.GKEStatus.UpstreamSpec.PrivateClusterConfig.EnablePrivateEndpoint {
			// Check to see if we can still use the control plane endpoint even though
			// the cluster has private-only access
			serviceToken, mustTunnel, err := e.generateSATokenWithPublicAPI(cluster)
			if err != nil {
				return cluster, err
			}
			if mustTunnel {
				cluster = cluster.DeepCopy()
				cluster.Status.GKEStatus.PrivateRequiresTunnel = &mustTunnel
				cluster.Status.ServiceAccountToken = serviceToken
				return e.clusterClient.Update(cluster)
			}
		}

		if cluster.Status.ServiceAccountToken == "" {
			cluster, err = e.generateAndSetServiceAccount(cluster)
			if err != nil {
				var statusErr error
				if strings.Contains(err.Error(), fmt.Sprintf(dialer.WaitForAgentError, cluster.Name)) {
					// In this case, the API endpoint is private and rancher is waiting for the import cluster command to be run.
					cluster, statusErr = e.setUnknown(cluster, apimgmtv3.ClusterConditionWaiting, "waiting for cluster agent to be deployed")
					if statusErr == nil {
						e.clusterEnqueueAfter(cluster.Name, enqueueTime)
					}
					return cluster, statusErr
				}
				cluster, statusErr = e.setFalse(cluster, apimgmtv3.ClusterConditionWaiting,
					fmt.Sprintf("failed to communicate with cluster: %v", err))
				if statusErr != nil {
					return cluster, statusErr
				}
				return cluster, err
			}
		}

		cluster, err = e.recordAppliedSpec(cluster)
		if err != nil {
			return cluster, err
		}
		return e.setTrue(cluster, apimgmtv3.ClusterConditionUpdated, "")
	case "updating":
		cluster, err = e.setTrue(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		e.clusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			logrus.Infof("waiting for cluster GKE [%s] to update", cluster.Name)
			return e.setUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		}
		logrus.Infof("waiting for cluster GKE [%s] update failure to be resolved", cluster.Name)
		return e.setFalse(cluster, apimgmtv3.ClusterConditionUpdated, failureMessage)
	default:
		if cluster.Spec.GKEConfig.Imported {
			cluster, err = e.setUnknown(cluster, apimgmtv3.ClusterConditionPending, "")
			if err != nil {
				return cluster, err
			}
			logrus.Infof("waiting for cluster import [%s] to start", cluster.Name)
		} else {
			logrus.Infof("waiting for cluster create [%s] to start", cluster.Name)
		}

		e.clusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			if cluster.Spec.GKEConfig.Imported {
				cluster, err = e.setUnknown(cluster, apimgmtv3.ClusterConditionPending, "")
				if err != nil {
					return cluster, err
				}
				logrus.Infof("waiting for cluster import [%s] to start", cluster.Name)
			} else {
				logrus.Infof("waiting for cluster create [%s] to start", cluster.Name)
			}
			return e.setUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		}
		logrus.Infof("waiting for cluster GKE [%s] pre-create failure to be resolved", cluster.Name)
		return e.setFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	}
}

// setInitialUpstreamSpec builds a view of the upstream cluster and adds it to the status of the cluster resource
func (e *gkeOperatorController) setInitialUpstreamSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	logrus.Infof("setting initial upstreamSpec on cluster [%s]", cluster.Name)
	cluster = cluster.DeepCopy()
	upstreamSpec, err := clusterupstreamrefresher.BuildGKEUpstreamSpec(e.secretsCache, cluster)
	if err != nil {
		return cluster, err
	}
	cluster.Status.GKEStatus.UpstreamSpec = upstreamSpec
	return e.clusterClient.Update(cluster)
}

// updateGKEClusterConfig updates the GKEClusterConfig object's spec with the cluster's GKEConfig if they are not equal.
func (e *gkeOperatorController) updateGKEClusterConfig(cluster *mgmtv3.Cluster, gkeClusterConfigDynamic *unstructured.Unstructured, spec map[string]interface{}) (*mgmtv3.Cluster, error) {
	list, err := e.dynamicClient.Namespace(namespace.GlobalNamespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return cluster, err
	}
	selector := fields.OneTermEqualSelector("metadata.name", cluster.Name)
	w, err := e.dynamicClient.Namespace(namespace.GlobalNamespace).Watch(context.TODO(), v1.ListOptions{ResourceVersion: list.GetResourceVersion(), FieldSelector: selector.String()})
	if err != nil {
		return cluster, err
	}
	gkeClusterConfigDynamic.Object["spec"] = spec
	gkeClusterConfigDynamic, err = e.dynamicClient.Namespace(namespace.GlobalNamespace).Update(context.TODO(), gkeClusterConfigDynamic, v1.UpdateOptions{})
	if err != nil {
		return cluster, err
	}

	// GKE cluster and node pool statuses are not always immediately updated. This cause the GKEConfig to
	// stay in "active" for a few seconds, causing the cluster to go back to "active".
	timeout := time.NewTimer(10 * time.Second)
	for {
		select {
		case event := <-w.ResultChan():
			gkeClusterConfigDynamic = event.Object.(*unstructured.Unstructured)
			status, _ := gkeClusterConfigDynamic.Object["status"].(map[string]interface{})
			if status["phase"] == "active" {
				continue
			}

			// this enqueue is necessary to ensure that the controller is reentered with the updating phase
			e.clusterEnqueueAfter(cluster.Name, enqueueTime)
			return e.setUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		case <-timeout.C:
			cluster, err = e.recordAppliedSpec(cluster)
			if err != nil {
				return cluster, err
			}
			return cluster, nil
		}
	}
}

// recordCAAndAPIEndpoint reads the GKEClusterConfig's secret once available. The CA cert and API endpoint are then copied to the cluster status.
func (e *gkeOperatorController) recordCAAndAPIEndpoint(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
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
		caSecret, err = e.secretsCache.Get(namespace.GlobalNamespace, cluster.Name)
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
		currentCluster, err = e.clusterClient.Get(cluster.Name, v1.GetOptions{})
		if err != nil {
			return err
		}
		currentCluster.Status.APIEndpoint = apiEndpoint
		currentCluster.Status.CACert = base64.StdEncoding.EncodeToString(caSecret.Data["ca"])
		currentCluster, err = e.clusterClient.Update(currentCluster)
		return err
	})

	return currentCluster, err
}

// generateAndSetServiceAccount uses the API endpoint and CA cert to generate a service account token. The token is then copied to the cluster status.
func (e *gkeOperatorController) generateAndSetServiceAccount(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {

	ctx := context.Background()
	ts, err := controller.GetTokenSource(ctx, e.secretsCache, cluster.Spec.GKEConfig)
	if err != nil {
		return nil, fmt.Errorf("error getting google credential from credentialContent: %w", err)
	}
	clusterDialer, err := e.clientDialer.ClusterDialer(cluster.Name)
	if err != nil {
		return cluster, err
	}
	saToken, err := generateSAToken(cluster.Status.APIEndpoint, cluster.Status.CACert, ts, clusterDialer)
	if err != nil {
		return cluster, fmt.Errorf("error generating service account token: %w", err)
	}

	cluster = cluster.DeepCopy()
	cluster.Status.ServiceAccountToken = saToken
	return e.clusterClient.Update(cluster)
}

// buildGKECCCreateObject returns an object that can be used with the kubernetes dynamic client to
// create an GKEClusterConfig that matches the spec contained in the cluster's GKEConfig.
func buildGKECCCreateObject(cluster *mgmtv3.Cluster) (*unstructured.Unstructured, error) {
	gkeClusterConfig := gkev1.GKEClusterConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       "GKEClusterConfig",
			APIVersion: gkeV1,
		},
		ObjectMeta: v1.ObjectMeta{
			Name: cluster.Name,
			OwnerReferences: []v1.OwnerReference{
				{
					Kind:       cluster.Kind,
					APIVersion: rbac.RancherManagementAPIVersion,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		Spec: *cluster.Spec.GKEConfig,
	}

	// convert GKE cluster config into unstructured object so it can be used with dynamic client
	gkeClusterConfigMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&gkeClusterConfig)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{
		Object: gkeClusterConfigMap,
	}, nil
}

// recordAppliedSpec sets the cluster's current spec as its appliedSpec
func (e *gkeOperatorController) recordAppliedSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if reflect.DeepEqual(cluster.Status.AppliedSpec.GKEConfig, cluster.Spec.GKEConfig) {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	cluster.Status.AppliedSpec.GKEConfig = cluster.Spec.GKEConfig
	return e.clusterClient.Update(cluster)
}

// deployGKEOperator looks for the rancher-gke-operator app in the cattle-system namespace, if not found it is deployed.
// If it is found but is outdated, the latest version is installed.
func (e *gkeOperatorController) deployGKEOperator() error {
	template, err := e.templateCache.Get(namespace.GlobalNamespace, gkeOperatorTemplate)
	if err != nil {
		return err
	}

	latestTemplateVersion, err := e.catalogManager.LatestAvailableTemplateVersion(template, "local")
	if err != nil {
		return err
	}

	latestVersionID := latestTemplateVersion.ExternalID

	systemProject, err := project.GetSystemProject(localCluster, e.projectCache)
	if err != nil {
		return err
	}

	systemProjectID := ref.Ref(systemProject)
	_, systemProjectName := ref.Parse(systemProjectID)

	valuesYaml, err := generateValuesYaml()
	if err != nil {
		return err
	}

	app, err := e.appLister.Get(systemProjectName, gkeOperator)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		logrus.Info("deploying GKE operator into local cluster's system project")
		creator, err := e.systemAccountManager.GetSystemUser(localCluster)
		if err != nil {
			return err
		}

		appProjectName, err := utils2.EnsureAppProjectName(e.nsClient, systemProjectName, localCluster, systemNS, creator.Name)
		if err != nil {
			return err
		}

		desiredApp := &apiprojv3.App{
			ObjectMeta: v1.ObjectMeta{
				Name:      gkeOperator,
				Namespace: systemProjectName,
				Annotations: map[string]string{
					rbac.CreatorIDAnn: creator.Name,
				},
			},
			Spec: apiprojv3.AppSpec{
				Description:     "Operator for provisioning GKE clusters",
				ExternalID:      latestVersionID,
				ProjectName:     appProjectName,
				TargetNamespace: systemNS,
			},
		}

		desiredApp.Spec.ValuesYaml = valuesYaml

		if _, err = e.appClient.Create(desiredApp); err != nil {
			return err
		}
	} else {
		if app.Spec.ExternalID == latestVersionID && app.Spec.ValuesYaml == valuesYaml {
			// app is up to date, no action needed
			return nil
		}
		logrus.Info("updating GKE operator in local cluster's system project")
		desiredApp := app.DeepCopy()
		desiredApp.Spec.ExternalID = latestVersionID
		desiredApp.Spec.ValuesYaml = valuesYaml
		// new version of gke-operator upgrade available, update app
		if _, err = e.appClient.Update(desiredApp); err != nil {
			return err
		}
	}

	return nil
}

func (e *gkeOperatorController) generateSATokenWithPublicAPI(cluster *mgmtv3.Cluster) (string, bool, error) {
	var publicAccess bool

	ctx := context.Background()
	ts, err := controller.GetTokenSource(ctx, e.secretsCache, cluster.Spec.GKEConfig)

	netDialer := net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	serviceToken, err := generateSAToken(cluster.Status.APIEndpoint, cluster.Status.CACert, ts, netDialer.DialContext)
	if err != nil {
		if strings.Contains(err.Error(), "dial tcp") {
			return "", true, nil
		}
	} else {
		publicAccess = false
	}

	return serviceToken, publicAccess, err
}

func generateSAToken(endpoint, ca string, ts oauth2.TokenSource, dialer typesDialer.Dialer) (string, error) {
	decodedCA, err := base64.StdEncoding.DecodeString(ca)
	if err != nil {
		return "", err
	}

	restConfig := &rest.Config{
		Host: endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: decodedCA,
		},
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			return &oauth2.Transport{
				Source: ts,
				Base:   rt,
			}
		},
		Dial: dialer,
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return "", fmt.Errorf("error creating clientset: %v", err)
	}

	return util.GenerateServiceAccountToken(clientset)
}

func (e *gkeOperatorController) setUnknown(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsUnknown(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.Unknown(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.clusterClient.Update(cluster)
	if err != nil {
		return cluster, fmt.Errorf("failed setting cluster [%s] condition %s unknown with message: %s", cluster.Name, condition, message)
	}
	return cluster, nil
}

func (e *gkeOperatorController) setTrue(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsTrue(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.True(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.clusterClient.Update(cluster)
	if err != nil {
		return cluster, fmt.Errorf("failed setting cluster [%s] condition %s true with message: %s", cluster.Name, condition, message)
	}
	return cluster, nil
}

func (e *gkeOperatorController) setFalse(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsFalse(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.False(cluster)
	condition.Message(cluster, message)
	var err error
	cluster, err = e.clusterClient.Update(cluster)
	if err != nil {
		return cluster, fmt.Errorf("failed setting cluster [%s] condition %s false with message: %s", cluster.Name, condition, message)
	}
	return cluster, nil
}

// generateValuesYaml generates a YAML string containing any
// necessary values to override defaults in values.yaml. If
// no defaults need to be overwritten, an empty string will
// be returned.
func generateValuesYaml() (string, error) {
	values := gkeOperatorValues{
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
