package eks

import (
	"context"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eks"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/eks-operator/utils"
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
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

const (
	eksAPIGroup         = "eks.cattle.io"
	eksV1               = "eks.cattle.io/v1"
	eksOperatorTemplate = "system-library-rancher-eks-operator"
	eksOperator         = "rancher-eks-operator"
	eksShortName        = "EKS"
	enqueueTime         = time.Second * 5
	importedAnno        = "eks.cattle.io/imported"
)

type eksOperatorController struct {
	clusteroperator.OperatorController
	secretClient corecontrollers.SecretClient
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext) {
	eksClusterConfigResource := schema.GroupVersionResource{
		Group:    eksAPIGroup,
		Version:  "v1",
		Resource: "eksclusterconfigs",
	}

	eksCCDynamicClient := mgmtCtx.DynamicClient.Resource(eksClusterConfigResource)
	e := &eksOperatorController{
		OperatorController: clusteroperator.OperatorController{
			ClusterEnqueueAfter:  wContext.Mgmt.Cluster().EnqueueAfter,
			SecretsCache:         wContext.Core.Secret().Cache(),
			Secrets:              mgmtCtx.Core.Secrets(""),
			TemplateCache:        wContext.Mgmt.CatalogTemplate().Cache(),
			ProjectCache:         wContext.Mgmt.Project().Cache(),
			AppLister:            mgmtCtx.Project.Apps("").Controller().Lister(),
			AppClient:            mgmtCtx.Project.Apps(""),
			NsClient:             mgmtCtx.Core.Namespaces(""),
			ClusterClient:        wContext.Mgmt.Cluster(),
			CatalogManager:       mgmtCtx.CatalogManager,
			SystemAccountManager: systemaccount.NewManager(mgmtCtx),
			DynamicClient:        eksCCDynamicClient,
			ClientDialer:         mgmtCtx.Dialer,
			Discovery:            wContext.K8s.Discovery(),
		},
		secretClient: wContext.Core.Secret(),
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "eks-operator-controller", e.onClusterChange)
}

func (e *eksOperatorController) onClusterChange(key string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return cluster, nil
	}

	if cluster.Spec.EKSConfig == nil {
		return cluster, nil
	}

	cluster, err := e.CheckCrdReady(cluster, "eks")
	if err != nil {
		return cluster, err
	}

	// set driver name
	if cluster.Status.Driver == "" {
		cluster = cluster.DeepCopy()
		cluster.Status.Driver = apimgmtv3.ClusterDriverEKS
		var err error
		cluster, err = e.ClusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	// get EKS Cluster Config, if it does not exist, create it
	eksClusterConfigDynamic, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).Get(context.TODO(), cluster.Name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return cluster, err
		}

		cluster, err = e.SetUnknown(cluster, apimgmtv3.ClusterConditionWaiting, "Waiting for API to be available")
		if err != nil {
			return cluster, err
		}

		eksClusterConfigDynamic, err = buildEKSCCCreateObject(cluster)
		if err != nil {
			return cluster, err
		}

		eksClusterConfigDynamic, err = e.DynamicClient.Namespace(namespace.GlobalNamespace).Create(context.TODO(), eksClusterConfigDynamic, v1.CreateOptions{})
		if err != nil {
			return cluster, err
		}

	}

	eksClusterConfigMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cluster.Spec.EKSConfig)
	if err != nil {
		return cluster, err
	}

	// check for changes between EKS spec on cluster and the EKS spec on the EKSClusterConfig object
	if !reflect.DeepEqual(eksClusterConfigMap, eksClusterConfigDynamic.Object["spec"]) {
		logrus.Infof("change detected for cluster [%s], updating EKSClusterConfig", cluster.Name)
		return e.updateEKSClusterConfig(cluster, eksClusterConfigDynamic, eksClusterConfigMap)
	}

	// get EKS Cluster Config's phase
	status, _ := eksClusterConfigDynamic.Object["status"].(map[string]interface{})
	phase, _ := status["phase"]
	failureMessage, _ := status["failureMessage"].(string)
	if strings.Contains(failureMessage, "403") {
		failureMessage = fmt.Sprintf("cannot access EKS, check cloud credential: %s", failureMessage)
	}
	switch phase {
	case "creating":
		if cluster.Status.EKSStatus.UpstreamSpec == nil {
			cluster, err = e.setInitialUpstreamSpec(cluster)
			if err != nil {
				if !notFound(err) {
					return cluster, err
				}
			}
			return cluster, nil
		}

		e.ClusterEnqueueAfter(cluster.Name, enqueueTime)
		if failureMessage == "" {
			logrus.Infof("waiting for cluster EKS [%s] to finish creating", cluster.Name)
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		}
		logrus.Infof("waiting for cluster EKS [%s] create failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	case "active":
		if cluster.Spec.EKSConfig.Imported {
			if cluster.Status.EKSStatus.UpstreamSpec == nil {
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

		if apimgmtv3.ClusterConditionUpdated.IsFalse(cluster) && strings.HasPrefix(apimgmtv3.ClusterConditionUpdated.GetMessage(cluster), "[Syncing error") {
			return cluster, fmt.Errorf(apimgmtv3.ClusterConditionUpdated.GetMessage(cluster))
		}

		if cluster.Status.EKSStatus.UpstreamSpec == nil {
			return cluster, fmt.Errorf("initial upstreamSpec on cluster [%s] has not been set, unable to continue", cluster.Name)
		}

		// EKS cluster must have at least one node to run cluster agent. The best way to verify
		// if a cluster has self-managed node or nodegroup is to check if the cluster agent was deployed.
		// Issue: https://github.com/rancher/eks-operator/issues/301
		addNgMessage := "Cluster must have at least one managed nodegroup or one self-managed node."
		noNodeGroupsOnSpec := len(cluster.Spec.EKSConfig.NodeGroups) == 0
		noNodeGroupsOnUpstreamSpec := len(cluster.Status.EKSStatus.UpstreamSpec.NodeGroups) == 0
		if !apimgmtv3.ClusterConditionAgentDeployed.IsTrue(cluster) &&
			((cluster.Spec.EKSConfig.NodeGroups != nil && noNodeGroupsOnSpec) ||
				(cluster.Spec.EKSConfig.NodeGroups == nil && noNodeGroupsOnUpstreamSpec)) {
			cluster, err = e.SetFalse(cluster, apimgmtv3.ClusterConditionWaiting, addNgMessage)
			if err != nil {
				return cluster, err
			}
		} else {
			if apimgmtv3.ClusterConditionWaiting.GetMessage(cluster) == addNgMessage {
				cluster = cluster.DeepCopy()
				apimgmtv3.ClusterConditionWaiting.Message(cluster, "Waiting for API to be available")
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

		// If there are no subnets it can be assumed that networking fields are not provided. In which case they
		// should be created by the eks-operator, and needs to be copied to the cluster object.
		if len(cluster.Status.EKSStatus.Subnets) == 0 {
			subnets, _ := status["subnets"].([]interface{})
			if len(subnets) != 0 {
				// network field have been generated and are ready to be copied
				virtualNetwork, _ := status["virtualNetwork"].(string)
				subnets, _ := status["subnets"].([]interface{})
				securityGroups, _ := status["securityGroups"].([]interface{})
				cluster = cluster.DeepCopy()

				// change fields on status to not be generated
				cluster.Status.EKSStatus.VirtualNetwork = virtualNetwork
				for _, val := range subnets {
					cluster.Status.EKSStatus.Subnets = append(cluster.Status.EKSStatus.Subnets, val.(string))
				}
				for _, val := range securityGroups {
					cluster.Status.EKSStatus.SecurityGroups = append(cluster.Status.EKSStatus.SecurityGroups, val.(string))
				}
				cluster, err = e.ClusterClient.Update(cluster)
				if err != nil {
					return cluster, err
				}
			}
		}

		if cluster.Status.APIEndpoint == "" {
			return e.RecordCAAndAPIEndpoint(cluster)
		}

		if cluster.Status.EKSStatus.PrivateRequiresTunnel == nil && !*cluster.Status.EKSStatus.UpstreamSpec.PublicAccess {
			// In this case, the API endpoint is private and it has not been determined if Rancher must tunnel to communicate with it.
			// Check to see if we can still use the public API endpoint even though
			// the cluster has private-only access
			serviceToken, mustTunnel, err := e.generateSATokenWithPublicAPI(cluster)
			if mustTunnel != nil {
				cluster = cluster.DeepCopy()
				cluster.Status.EKSStatus.PrivateRequiresTunnel = mustTunnel
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
			if err != nil {
				return cluster, err
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

		managedLaunchTemplateID, _ := status["managedLaunchTemplateID"].(string)
		if managedLaunchTemplateID != "" && cluster.Status.EKSStatus.ManagedLaunchTemplateID != managedLaunchTemplateID {
			cluster = cluster.DeepCopy()
			cluster.Status.EKSStatus.ManagedLaunchTemplateID = managedLaunchTemplateID
			cluster, err = e.ClusterClient.Update(cluster)
			if err != nil {
				return cluster, err
			}
		}

		managedLaunchTemplateVersions, _ := status["managedLaunchTemplateVersions"].(map[string]interface{})
		if !reflect.DeepEqual(cluster.Status.EKSStatus.ManagedLaunchTemplateVersions, managedLaunchTemplateVersions) {
			managedLaunchTemplateVersionsToString := make(map[string]string, len(managedLaunchTemplateVersions))
			for key, value := range managedLaunchTemplateVersions {
				managedLaunchTemplateVersionsToString[key] = value.(string)
			}
			cluster = cluster.DeepCopy()
			cluster.Status.EKSStatus.ManagedLaunchTemplateVersions = managedLaunchTemplateVersionsToString
			cluster, err = e.ClusterClient.Update(cluster)
			if err != nil {
				return cluster, err
			}
		}

		generatedNodeRole, _ := status["generatedNodeRole"].(string)
		if generatedNodeRole != "" && cluster.Status.EKSStatus.GeneratedNodeRole != generatedNodeRole {
			cluster = cluster.DeepCopy()
			cluster.Status.EKSStatus.GeneratedNodeRole = generatedNodeRole
			cluster, err = e.ClusterClient.Update(cluster)
			if err != nil {
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
			logrus.Infof("waiting for cluster EKS [%s] to update", cluster.Name)
			return e.SetUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		}
		logrus.Infof("waiting for cluster EKS [%s] update failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionUpdated, failureMessage)
	default:
		if cluster.Spec.EKSConfig.Imported {
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
			if cluster.Spec.EKSConfig.Imported {
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
		logrus.Infof("waiting for cluster EKS [%s] pre-create failure to be resolved", cluster.Name)
		return e.SetFalse(cluster, apimgmtv3.ClusterConditionProvisioned, failureMessage)
	}
}

func (e *eksOperatorController) setInitialUpstreamSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	logrus.Infof("setting initial upstreamSpec on cluster [%s]", cluster.Name)
	cluster = cluster.DeepCopy()
	upstreamSpec, err := clusterupstreamrefresher.BuildEKSUpstreamSpec(e.secretClient, cluster)
	if err != nil {
		logrus.Warnf("failed to set initial upstreamSpec on cluster [%s]: %v", cluster.Name, err)
		return cluster, err
	}
	cluster.Status.EKSStatus.UpstreamSpec = upstreamSpec
	return e.ClusterClient.Update(cluster)
}

// updateEKSClusterConfig updates the EKSClusterConfig object's spec with the cluster's EKSConfig if they are not equal..
func (e *eksOperatorController) updateEKSClusterConfig(cluster *mgmtv3.Cluster, eksClusterConfigDynamic *unstructured.Unstructured, spec map[string]interface{}) (*mgmtv3.Cluster, error) {
	list, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return cluster, err
	}
	selector := fields.OneTermEqualSelector("metadata.name", cluster.Name)
	w, err := e.DynamicClient.Namespace(namespace.GlobalNamespace).Watch(context.TODO(), v1.ListOptions{ResourceVersion: list.GetResourceVersion(), FieldSelector: selector.String()})
	if err != nil {
		return cluster, err
	}
	eksClusterConfigDynamic.Object["spec"] = spec
	eksClusterConfigDynamic, err = e.DynamicClient.Namespace(namespace.GlobalNamespace).Update(context.TODO(), eksClusterConfigDynamic, v1.UpdateOptions{})
	if err != nil {
		return cluster, err
	}

	// EKS cluster and node group statuses are not always immediately updated. This cause the EKSConfig to
	// stay in "active" for a few seconds, causing the cluster to go back to "active".
	timeout := time.NewTimer(10 * time.Second)
	for {
		select {
		case event := <-w.ResultChan():
			eksClusterConfigDynamic = event.Object.(*unstructured.Unstructured)
			status, _ := eksClusterConfigDynamic.Object["status"].(map[string]interface{})
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
func (e *eksOperatorController) generateAndSetServiceAccount(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
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
		return cluster, err
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

// buildEKSCCCreateObject returns an object that can be used with the kubernetes dynamic client to
// create an EKSClusterConfig that matches the spec contained in the cluster's EKSConfig.
func buildEKSCCCreateObject(cluster *mgmtv3.Cluster) (*unstructured.Unstructured, error) {
	eksClusterConfig := eksv1.EKSClusterConfig{
		TypeMeta: v1.TypeMeta{
			Kind:       "EKSClusterConfig",
			APIVersion: eksV1,
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
		Spec: *cluster.Spec.EKSConfig,
	}

	// convert EKS cluster config into unstructured object so it can be used with dynamic client
	eksClusterConfigMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&eksClusterConfig)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{
		Object: eksClusterConfigMap,
	}, nil
}

// recordAppliedSpec sets the cluster's current spec as its appliedSpec
func (e *eksOperatorController) recordAppliedSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if reflect.DeepEqual(cluster.Status.AppliedSpec.EKSConfig, cluster.Spec.EKSConfig) {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	cluster.Status.AppliedSpec.EKSConfig = cluster.Spec.EKSConfig
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
func (e *eksOperatorController) generateSATokenWithPublicAPI(cluster *mgmtv3.Cluster) (string, *bool, error) {
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
		var dnsError *net.DNSError
		if stderrors.As(err, &dnsError) && !dnsError.IsTemporary {
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

func (e *eksOperatorController) getAWSSession(cluster *mgmtv3.Cluster) (*session.Session, error) {
	awsConfig := &aws.Config{}
	eksConfig := cluster.Spec.EKSConfig

	if region := eksConfig.Region; region != "" {
		awsConfig.Region = aws.String(region)
	}

	ns, id := utils.Parse(eksConfig.AmazonCredentialSecret)
	if amazonCredentialSecret := eksConfig.AmazonCredentialSecret; amazonCredentialSecret != "" {
		secret, err := e.SecretsCache.Get(ns, id)
		if err != nil {
			return nil, fmt.Errorf("error getting secret %s/%s: %w", ns, id, err)
		}

		accessKeyBytes := secret.Data["amazonec2credentialConfig-accessKey"]
		secretKeyBytes := secret.Data["amazonec2credentialConfig-secretKey"]
		if accessKeyBytes == nil || secretKeyBytes == nil {
			return nil, fmt.Errorf("invalid aws cloud credential")
		}

		accessKey := string(accessKeyBytes)
		secretKey := string(secretKeyBytes)

		awsConfig.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("error getting new aws session: %w", err)
	}
	return sess, nil
}

func (e *eksOperatorController) getAccessToken(cluster *mgmtv3.Cluster) (string, error) {
	sess, err := e.getAWSSession(cluster)
	if err != nil {
		return "", err
	}
	generator, err := token.NewGenerator(false, false)
	if err != nil {
		return "", err
	}

	awsToken, err := generator.GetWithOptions(&token.GetTokenOptions{
		Session:   sess,
		ClusterID: cluster.Spec.EKSConfig.DisplayName,
	})
	if err != nil {
		return "", err
	}

	return awsToken.Token, nil
}

func (e *eksOperatorController) getRestConfig(cluster *mgmtv3.Cluster) (*rest.Config, error) {
	accessToken, err := e.getAccessToken(cluster)
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
		UserAgent:   util.UserAgentForCluster(cluster),
		BearerToken: accessToken,
	}, nil
}

func notFound(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		return awsErr.Code() == eks.ErrCodeResourceNotFoundException
	}
	return false
}
