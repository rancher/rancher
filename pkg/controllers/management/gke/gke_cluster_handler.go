package gke

import (
	"context"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/rancher/gke-operator/controller"
	gkev1 "github.com/rancher/gke-operator/pkg/apis/gke.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/clusteroperator"
	"github.com/rancher/rancher/pkg/controllers/management/clusterupstreamrefresher"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	"github.com/rancher/rancher/pkg/dialer"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/util"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
)

const (
	gkeAPIGroup         = "gke.cattle.io"
	gkeV1               = "gke.cattle.io/v1"
	gkeOperatorTemplate = "system-library-rancher-gke-operator"
	gkeOperator         = "rancher-gke-operator"
	gkeShortName        = "GKE"
	enqueueTime         = time.Second * 5
)

type gkeOperatorController struct {
	clusteroperator.OperatorController
	secretClient corecontrollers.SecretClient
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext) {
	gkeClusterConfigResource := schema.GroupVersionResource{
		Group:    gkeAPIGroup,
		Version:  "v1",
		Resource: "gkeclusterconfigs",
	}

	gkeCCDynamicClient := mgmtCtx.DynamicClient.Resource(gkeClusterConfigResource)
	e := &gkeOperatorController{
		OperatorController: clusteroperator.OperatorController{
			ClusterEnqueueAfter:  wContext.Mgmt.Cluster().EnqueueAfter,
			Secrets:              mgmtCtx.Core.Secrets(""),
			SecretsCache:         wContext.Core.Secret().Cache(),
			TemplateCache:        wContext.Mgmt.CatalogTemplate().Cache(),
			ProjectCache:         wContext.Mgmt.Project().Cache(),
			AppLister:            mgmtCtx.Project.Apps("").Controller().Lister(),
			AppClient:            mgmtCtx.Project.Apps(""),
			NsClient:             mgmtCtx.Core.Namespaces(""),
			ClusterClient:        wContext.Mgmt.Cluster(),
			CatalogManager:       mgmtCtx.CatalogManager,
			SystemAccountManager: systemaccount.NewManager(mgmtCtx),
			DynamicClient:        gkeCCDynamicClient,
			ClientDialer:         mgmtCtx.Dialer,
			Discovery:            wContext.K8s.Discovery(),
		},
		secretClient: wContext.Core.Secret(),
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "gke-operator-controller", e.onClusterChange)
}

func (e *gkeOperatorController) onClusterChange(key string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Spec.GKEConfig == nil {
		return cluster, nil
	}

	// set driver name
	if cluster.Status.Driver == "" {
		cluster = cluster.DeepCopy()
		cluster.Status.Driver = apimgmtv3.ClusterDriverGKE
		var err error
		cluster, err = e.ClusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	cluster, err := e.CheckCrdReady(cluster, "gke")
	if err != nil {
		return cluster, err
	}

	// get gke Cluster Config, if it does not exist, create it
	gkeClusterConfigDynamic, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).Get(context.TODO(), cluster.Name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return cluster, err
		}

		cluster, err = e.SetUnknown(cluster, apimgmtv3.ClusterConditionWaiting, "Waiting for API to be available")
		if err != nil {
			return cluster, err
		}

		gkeClusterConfigDynamic, err = buildGKECCCreateObject(cluster)
		if err != nil {
			return cluster, err
		}

		gkeClusterConfigDynamic, err = e.DynamicClient.Namespace(namespace.GlobalNamespace).Create(context.TODO(), gkeClusterConfigDynamic, v1.CreateOptions{})
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
		if cluster.Status.GKEStatus.UpstreamSpec == nil {
			cluster, err = e.setInitialUpstreamSpec(cluster)
			if err != nil {
				return cluster, err
			}
			return cluster, nil
		}

		e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			logrus.Infof("waiting for cluster GKE [%s] to finish creating", cluster.Name)
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		}
		logrus.Infof("waiting for cluster GKE [%s] create failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	case "active":
		if cluster.Spec.GKEConfig.Imported {
			if cluster.Status.GKEStatus.UpstreamSpec == nil {
				// non imported clusters will have already had upstream spec set
				return e.setInitialUpstreamSpec(cluster)
			}

			if apimgmtv3.ClusterConditionPending.IsUnknown(cluster) {
				cluster = cluster.DeepCopy()
				apimgmtv3.ClusterConditionPending.True(cluster)
				cluster, err = e.ClusterClient.Update(cluster)
				if err != nil {
					return cluster, err
				}
			}
		}

		cluster, err = e.SetTrue(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		if cluster.Status.APIEndpoint == "" {
			return e.RecordCAAndAPIEndpoint(cluster)
		}

		if cluster.Status.GKEStatus.PrivateRequiresTunnel == nil &&
			cluster.Status.GKEStatus.UpstreamSpec.PrivateClusterConfig != nil &&
			cluster.Status.GKEStatus.UpstreamSpec.PrivateClusterConfig.EnablePrivateEndpoint {
			// In this case, the API endpoint is private and it has not been determined if Rancher must tunnel to communicate with it.
			// Check to see if we can still use the control plane endpoint even though
			// the cluster has private-only access
			serviceToken, mustTunnel, err := e.generateSATokenWithPublicAPI(cluster)
			if err != nil {
				return cluster, err
			}
			if mustTunnel != nil {
				cluster = cluster.DeepCopy()
				cluster.Status.GKEStatus.PrivateRequiresTunnel = mustTunnel
				if serviceToken != "" {
					secret, err := secretmigrator.NewMigrator(e.SecretsCache, e.Secrets).CreateOrUpdateServiceAccountTokenSecret(cluster.Status.ServiceAccountTokenSecret, serviceToken, cluster)
					if err != nil {
						return cluster, err
					}
					if secret == nil {
						logrus.Debugf("Empty service account token secret returned for cluster [%s]", cluster.Name)
						return cluster, fmt.Errorf("failed to create or update service account token secret, secret can't be empty")
					}
					cluster.Status.ServiceAccountTokenSecret = secret.Name
					cluster.Status.ServiceAccountToken = ""
				}
				return e.ClusterClient.Update(cluster)
			}
		}

		if cluster.Status.ServiceAccountTokenSecret == "" {
			cluster, err = e.generateAndSetServiceAccount(cluster)
			if err != nil {
				var statusErr error
				if err == dialer.ErrAgentDisconnected {
					// In this case, the API endpoint is private and rancher is waiting for the import cluster command to be run.
					cluster, statusErr = e.SetUnknown(cluster, apimgmtv3.ClusterConditionWaiting, "waiting for cluster agent to be deployed")
					if statusErr == nil {
						e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
					}
					return cluster, statusErr
				}
				cluster, statusErr = e.SetFalse(cluster, apimgmtv3.ClusterConditionWaiting,
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
		return e.SetTrue(cluster, apimgmtv3.ClusterConditionUpdated, "")
	case "updating":
		cluster, err = e.SetTrue(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			logrus.Infof("waiting for cluster GKE [%s] to update", cluster.Name)

			// If the HealthSyncer runs while upgrading a zonal cluster, the control plane may not be reachable.
			// This adds additional context to the error message to help explain that this is normal.
			readyMsg := apimgmtv3.ClusterConditionReady.GetMessage(cluster)
			helpMsg := ": control plane may be unavailable while it is being upgraded"
			if apimgmtv3.ClusterConditionReady.IsFalse(cluster) && strings.Contains(readyMsg, "connect: connection refused") && !strings.Contains(readyMsg, helpMsg) {
				msg := apimgmtv3.ClusterConditionReady.GetMessage(cluster) + helpMsg
				// return here; ClusterConditionUpdated is most likely already set, and
				// if not will be set on the next loop
				return e.SetFalse(cluster, apimgmtv3.ClusterConditionReady, msg)
			}

			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		}
		logrus.Infof("waiting for cluster GKE [%s] update failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionUpdated, failureMessage)
	default:
		if cluster.Spec.GKEConfig.Imported {
			cluster, err = e.SetUnknown(cluster, apimgmtv3.ClusterConditionPending, "")
			if err != nil {
				return cluster, err
			}
			logrus.Infof("waiting for cluster import [%s] to start", cluster.Name)
		} else {
			logrus.Infof("waiting for cluster create [%s] to start", cluster.Name)
		}

		e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			if cluster.Spec.GKEConfig.Imported {
				cluster, err = e.SetUnknown(cluster, apimgmtv3.ClusterConditionPending, "")
				if err != nil {
					return cluster, err
				}
				logrus.Infof("waiting for cluster import [%s] to start", cluster.Name)
			} else {
				logrus.Infof("waiting for cluster create [%s] to start", cluster.Name)
			}
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		}
		logrus.Infof("waiting for cluster GKE [%s] pre-create failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	}
}

// setInitialUpstreamSpec builds a view of the upstream cluster and adds it to the status of the cluster resource
func (e *gkeOperatorController) setInitialUpstreamSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	logrus.Infof("setting initial upstreamSpec on cluster [%s]", cluster.Name)
	cluster = cluster.DeepCopy()
	upstreamSpec, err := clusterupstreamrefresher.BuildGKEUpstreamSpec(e.SecretsCache, e.secretClient, cluster)
	if err != nil {
		return cluster, err
	}
	cluster.Status.GKEStatus.UpstreamSpec = upstreamSpec
	return e.ClusterClient.Update(cluster)
}

// updateGKEClusterConfig updates the GKEClusterConfig object's spec with the cluster's GKEConfig if they are not equal.
func (e *gkeOperatorController) updateGKEClusterConfig(cluster *mgmtv3.Cluster, gkeClusterConfigDynamic *unstructured.Unstructured, spec map[string]interface{}) (*mgmtv3.Cluster, error) {
	list, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return cluster, err
	}
	selector := fields.OneTermEqualSelector("metadata.name", cluster.Name)
	w, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).Watch(context.TODO(), v1.ListOptions{ResourceVersion: list.GetResourceVersion(), FieldSelector: selector.String()})
	if err != nil {
		return cluster, err
	}
	gkeClusterConfigDynamic.Object["spec"] = spec
	gkeClusterConfigDynamic, err = e.DynamicClient.Namespace(namespace.GlobalNamespace).Update(context.TODO(), gkeClusterConfigDynamic, v1.UpdateOptions{})
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
			e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		case <-timeout.C:
			cluster, err = e.recordAppliedSpec(cluster)
			if err != nil {
				return cluster, err
			}
			return cluster, nil
		}
	}
}

// generateAndSetServiceAccount uses the API endpoint and CA cert to generate a service account token. The token is then copied to the cluster status.
func (e *gkeOperatorController) generateAndSetServiceAccount(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	clusterDialer, err := e.ClientDialer.ClusterDialHolder(cluster.Name, true)
	if err != nil {
		return cluster, err
	}

	restConfig, err := e.getRestConfig(cluster)
	if err != nil {
		return cluster, err
	}
	clientset, err := clusteroperator.NewClientSetForConfig(restConfig, clusteroperator.WithDialHolder(clusterDialer))
	if err != nil {
		return nil, fmt.Errorf("error creating clientset for cluster %s: %w", cluster.Name, err)
	}

	saToken, err := util.GenerateServiceAccountToken(clientset, cluster.Name)
	if err != nil {
		return cluster, fmt.Errorf("error generating service account token: %w", err)
	}

	cluster = cluster.DeepCopy()
	secret, err := secretmigrator.NewMigrator(e.SecretsCache, e.Secrets).CreateOrUpdateServiceAccountTokenSecret(cluster.Status.ServiceAccountTokenSecret, saToken, cluster)
	if err != nil {
		return nil, err
	}
	cluster.Status.ServiceAccountTokenSecret = secret.Name
	cluster.Status.ServiceAccountToken = ""
	return e.ClusterClient.Update(cluster)
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
	return e.ClusterClient.Update(cluster)
}

var publicDialer = &transport.DialHolder{
	Dial: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
}

// generateSATokenWithPublicAPI tries to get a service account token from the cluster using the public API endpoint.
// This function is called if the cluster has only privateEndpoint enabled and not publicly available.
// If Rancher is able to communicate with the cluster through its API endpoint even though it is private, then this function will retrieve
// a service account token and the *bool returned will refer to a false value (doesn't have to tunnel).
//
// If the Rancher server cannot connect to the cluster's API endpoint, then one of the two errors below will happen.
// In this case, we know that Rancher must use the cluster agent tunnel for communication. This function will return an empty service account token,
// and the *bool return value will refer to a true value (must tunnel).
//
// If an error different from the two below occur, then the *bool return value will be nil, indicating that Rancher was not able to determine if
// tunneling is required to communicate with the cluster.
func (e *gkeOperatorController) generateSATokenWithPublicAPI(cluster *mgmtv3.Cluster) (string, *bool, error) {
	restConfig, err := e.getRestConfig(cluster)
	if err != nil {
		return "", nil, err
	}
	clientset, err := clusteroperator.NewClientSetForConfig(restConfig, clusteroperator.WithDialHolder(publicDialer))
	if err != nil {
		return "", nil, fmt.Errorf("error creating clientset for cluster %s: %w", cluster.Name, err)
	}

	requiresTunnel := new(bool)
	serviceToken, err := util.GenerateServiceAccountToken(clientset, cluster.Name)
	if err != nil {
		*requiresTunnel = true
		if strings.Contains(err.Error(), "dial tcp") {
			return "", requiresTunnel, nil
		}

		// In the existence of a proxy, it may be the case that the following error occurs,
		// in which case rancher should use the tunnel connection to communicate with the cluster.
		var urlError *url.Error
		if stderrors.As(err, &urlError) && urlError.Timeout() {
			return "", requiresTunnel, nil
		}

		// Not able to determine if tunneling is required.
		requiresTunnel = nil
	}

	return serviceToken, requiresTunnel, err
}

func (e *gkeOperatorController) getRestConfig(cluster *mgmtv3.Cluster) (*rest.Config, error) {
	ctx := context.Background()
	ts, err := controller.GetTokenSource(ctx, e.secretClient, cluster.Spec.GKEConfig)
	if err != nil {
		return nil, err
	}

	decodedCA, err := base64.StdEncoding.DecodeString(cluster.Status.CACert)
	if err != nil {
		return nil, err
	}

	return &rest.Config{
		Host: cluster.Status.APIEndpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: decodedCA,
		},
		UserAgent: util.UserAgentForCluster(cluster),
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			return &oauth2.Transport{
				Source: ts,
				Base:   rt,
			}
		},
	}, nil
}
