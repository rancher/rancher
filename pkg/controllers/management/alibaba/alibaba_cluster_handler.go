package alibaba

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"

	stderrors "errors"

	"github.com/rancher/ali-operator/controller"
	aliv1 "github.com/rancher/ali-operator/pkg/apis/ali.cattle.io/v1"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/controllers/management/clusteroperator"
	"github.com/rancher/rancher/pkg/controllers/management/clusterupstreamrefresher"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/controllers/management/secretmigrator"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/kontainer-engine/drivers/util"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	wranglerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

const (
	aliAPIGroup = "ali.cattle.io"
	aliV1       = "ali.cattle.io/v1"
	enqueueTime = time.Second * 5
)

type aliOperatorController struct {
	clusteroperator.OperatorController
	secretClient corecontrollers.SecretClient
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext) {
	aliClusterConfigResource := schema.GroupVersionResource{
		Group:    aliAPIGroup,
		Version:  "v1",
		Resource: "aliclusterconfigs",
	}

	aliCCDynamicClient := mgmtCtx.DynamicClient.Resource(aliClusterConfigResource)
	e := &aliOperatorController{
		OperatorController: clusteroperator.OperatorController{
			ClusterEnqueueAfter:  wContext.Mgmt.Cluster().EnqueueAfter,
			SecretsCache:         wContext.Core.Secret().Cache(),
			Secrets:              mgmtCtx.Core.Secrets(""),
			ProjectCache:         wContext.Mgmt.Project().Cache(),
			NsClient:             mgmtCtx.Core.Namespaces(""),
			ClusterClient:        wContext.Mgmt.Cluster(),
			SystemAccountManager: systemaccount.NewManager(mgmtCtx),
			DynamicClient:        aliCCDynamicClient,
			ClientDialer:         mgmtCtx.Dialer,
			Discovery:            wContext.K8s.Discovery(),
		},
		secretClient: wContext.Core.Secret(),
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "ali-operator-controller", e.onClusterChange)
}

func (e *aliOperatorController) onClusterChange(_ string, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil || cluster.Spec.AliConfig == nil {
		return cluster, nil
	}

	// set driver name
	if cluster.Status.Driver == "" {
		cluster = cluster.DeepCopy()
		cluster.Status.Driver = apimgmtv3.ClusterDriverAlibaba
		return e.ClusterClient.Update(cluster)
	}

	cluster, err := e.CheckCrdReady(cluster, "ali")
	if err != nil {
		return cluster, err
	}

	// get ali Cluster Config, if it does not exist, create it
	aliClusterConfigDynamic, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).Get(context.TODO(), cluster.Name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return cluster, err
		}

		cluster, err = e.SetUnknown(cluster, apimgmtv3.ClusterConditionWaiting, "Waiting for API to be available")
		if err != nil {
			return cluster, err
		}

		aliClusterConfigDynamic, err = buildAliCCCreateObject(cluster)
		if err != nil {
			return cluster, err
		}

		aliClusterConfigDynamic, err = e.DynamicClient.Namespace(namespace.GlobalNamespace).Create(context.TODO(), aliClusterConfigDynamic, v1.CreateOptions{})
		if err != nil {
			return cluster, err
		}
	}

	aliClusterConfigMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cluster.Spec.AliConfig)
	if err != nil {
		return cluster, err
	}

	// fixAliConfig update the clusterID in clusterSpec (cluster.Spec.AliConfig.ClusterID) for cluster that are created with rancher
	// since this field is only available in AliClusterConfig object after ali-operator creates the cluster, it is needed to be fixed on clusterSpec
	e.fixAliConfig(cluster, aliClusterConfigDynamic)

	// check for changes between ali spec on cluster and the ali spec on the aliClusterConfig object
	if !reflect.DeepEqual(aliClusterConfigMap, aliClusterConfigDynamic.Object["spec"]) {
		logrus.Infof("change detected for cluster [%s], updating AliClusterConfig", cluster.Name)
		return e.updateAliClusterConfig(cluster, aliClusterConfigDynamic, aliClusterConfigMap)
	}

	// get ali Cluster Config's phase
	status, _ := aliClusterConfigDynamic.Object["status"].(map[string]interface{})
	phase, _ := status["phase"]
	failureMessage, _ := status["failureMessage"].(string)
	if strings.Contains(failureMessage, "403") {
		failureMessage = fmt.Sprintf("cannot access alibaba cloud, check cloud credential: %s", failureMessage)
	}

	switch phase {
	case "creating":
		if cluster.Status.AliStatus.UpstreamSpec == nil {
			cluster, err = e.setInitialUpstreamSpec(cluster)
			if err != nil {
				return cluster, err
			}
			return cluster, nil
		}

		e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			logrus.Infof("waiting for cluster ACK [%s] to finish creating", cluster.Name)
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		}
		logrus.Infof("waiting for cluster ACK [%s] create failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	case "active":
		if cluster.Spec.AliConfig.Imported {
			if cluster.Status.AliStatus.UpstreamSpec == nil {
				// non imported clusters will have already had upstream spec set
				return e.setInitialUpstreamSpec(cluster)
			}

			if apimgmtv3.ClusterConditionPending.IsUnknown(cluster) {
				cluster, err = e.SetTrue(cluster, apimgmtv3.ClusterConditionPending, "")
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

		// In this case, the API endpoint is private and it has not been determined if Rancher must tunnel to communicate with it.
		if cluster.Status.AliStatus.PrivateRequiresTunnel == nil &&
			!(cluster.Status.AliStatus.UpstreamSpec != nil &&
				cluster.Status.AliStatus.UpstreamSpec.EndpointPublicAccess != nil &&
				*cluster.Status.AliStatus.UpstreamSpec.EndpointPublicAccess) {
			// Check to see if we can still use the control plane endpoint even though
			// the cluster has private-only access
			serviceToken, mustTunnel, err := e.generateSATokenWithPublicAPI(cluster)
			if err != nil {
				return cluster, err
			}
			if mustTunnel != nil {
				cluster = cluster.DeepCopy()
				cluster.Status.AliStatus.PrivateRequiresTunnel = mustTunnel
				if serviceToken != "" {
					secret, err := secretmigrator.NewMigrator(e.SecretsCache, e.Secrets).CreateOrUpdateServiceAccountTokenSecret(cluster.Status.ServiceAccountTokenSecret, serviceToken, cluster)
					if err != nil {
						return cluster, err
					}
					cluster.Status.ServiceAccountTokenSecret = secret.Name
					cluster.Status.ServiceAccountToken = ""
				}
				return e.ClusterClient.Update(cluster)
			}
		}

		if cluster.Status.ServiceAccountTokenSecret == "" {
			cluster, err = e.generateAndSetServiceAccount(e.SecretsCache, cluster)
			if err != nil {
				var statusErr error
				if strings.Contains(err.Error(), fmt.Sprintf(dialer.WaitForAgentError, cluster.Name)) {
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
			logrus.Infof("waiting for cluster ACK [%s] to update", cluster.Name)
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		}
		logrus.Infof("waiting for cluster ACK [%s] update failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionUpdated, failureMessage)
	default:
		if cluster.Spec.AliConfig.Imported {
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
			if cluster.Spec.AliConfig.Imported {
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
		logrus.Infof("waiting for cluster ACK [%s] pre-create failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	}
}

// buildAliCCCreateObject returns an object that can be used with the kubernetes dynamic client to
// create an AliClusterConfig that matches the spec contained in the cluster's AliConfig.
func buildAliCCCreateObject(cluster *apimgmtv3.Cluster) (*unstructured.Unstructured, error) {
	ackClusterConfig := aliv1.AliClusterConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       "AliClusterConfig",
			APIVersion: aliV1,
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
		Spec: *cluster.Spec.AliConfig,
	}

	// convert ACK cluster config into unstructured object so it can be used with dynamic client
	ackClusterConfigMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ackClusterConfig)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{
		Object: ackClusterConfigMap,
	}, nil
}

// updateAliClusterConfig updates the AliClusterConfig object's spec with the cluster's AliConfig if they are not equal..
func (e *aliOperatorController) updateAliClusterConfig(cluster *apimgmtv3.Cluster, aliClusterConfigDynamic *unstructured.Unstructured, spec map[string]interface{}) (*apimgmtv3.Cluster, error) {
	list, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return cluster, err
	}
	selector := fields.OneTermEqualSelector("metadata.name", cluster.Name)
	w, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).Watch(context.TODO(), v1.ListOptions{ResourceVersion: list.GetResourceVersion(), FieldSelector: selector.String()})
	if err != nil {
		return cluster, err
	}
	aliClusterConfigDynamic.Object["spec"] = spec
	aliClusterConfigDynamic, err = e.DynamicClient.Namespace(namespace.GlobalNamespace).Update(context.TODO(), aliClusterConfigDynamic, v1.UpdateOptions{})
	if err != nil {
		return cluster, err
	}

	// ACK cluster and node group statuses are not always immediately updated. This cause the AliConfig to
	// stay in "active" for a few seconds, causing the cluster to go back to "active".
	timeout := time.NewTimer(10 * time.Second)
	for {
		select {
		case event := <-w.ResultChan():
			aliClusterConfigDynamic = event.Object.(*unstructured.Unstructured)
			status, _ := aliClusterConfigDynamic.Object["status"].(map[string]interface{})
			if status["phase"] == "active" {
				continue
			}

			// this enqueue is necessary to ensure that the controller is reentered with the updating phase
			e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		case <-timeout.C:
			return cluster, nil
		}
	}
}

func (e *aliOperatorController) setInitialUpstreamSpec(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	logrus.Infof("setting initial upstreamSpec on cluster [%s]", cluster.Name)
	cluster = cluster.DeepCopy()
	upstreamSpec, err := clusterupstreamrefresher.BuildAlibabaUpstreamSpec(e.SecretsCache, cluster)
	if err != nil {
		return cluster, err
	}
	cluster.Status.AliStatus.UpstreamSpec = upstreamSpec
	return e.ClusterClient.Update(cluster)
}

// recordAppliedSpec sets the cluster's current spec as its appliedSpec
func (e *aliOperatorController) recordAppliedSpec(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	cluster = cluster.DeepCopy()
	cluster.Status.AppliedSpec.AliConfig = cluster.Spec.AliConfig
	return e.ClusterClient.Update(cluster)
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
func (e *aliOperatorController) generateSATokenWithPublicAPI(cluster *apimgmtv3.Cluster) (string, *bool, error) {
	var publicDialer = &transport.DialHolder{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}

	kubConfig, err := controller.GetUserConfig(e.SecretsCache, cluster.Spec.AliConfig)
	if err != nil {
		return "", nil, err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(*kubConfig.Config))
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

// generateAndSetServiceAccount uses the API endpoint and CA cert to generate a service account token. The token is then copied to the cluster status.
func (e *aliOperatorController) generateAndSetServiceAccount(secretsCache wranglerv1.SecretCache, cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	clusterDialer, err := e.ClientDialer.ClusterDialHolder(cluster.Name, true)
	if err != nil {
		return cluster, err
	}

	kubConfig, err := controller.GetUserConfig(secretsCache, cluster.Spec.AliConfig)
	if err != nil {
		return cluster, err
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(*kubConfig.Config))
	if err != nil {
		return cluster, err
	}
	clientset, err := clusteroperator.NewClientSetForConfig(restConfig, clusteroperator.WithDialHolder(clusterDialer))
	if err != nil {
		return cluster, err
	}

	saToken, err := util.GenerateServiceAccountToken(clientset, cluster.Name)
	if err != nil {
		return cluster, fmt.Errorf("error getting service account token: %v", err)
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

func (e *aliOperatorController) fixAliConfig(cluster *apimgmtv3.Cluster, aliClusterConfigDynamic *unstructured.Unstructured) {
	aliConfigSpec, exist := aliClusterConfigDynamic.Object["spec"]
	if exist && aliConfigSpec != nil {
		spec := aliConfigSpec.(map[string]interface{})
		clusterID, exists := spec["clusterId"]
		if exists && clusterID != nil {
			cluster.Spec.AliConfig.ClusterID = clusterID.(string)
		}
	}
}
