package eks

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	eksv1 "github.com/rancher/eks-operator/pkg/apis/eks.cattle.io/v1"
	"github.com/rancher/norman/condition"
	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apiprojv3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	utils2 "github.com/rancher/rancher/pkg/app"
	"github.com/rancher/rancher/pkg/catalog/utils"
	"github.com/rancher/rancher/pkg/controllers/management/rbac"
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
	"github.com/rancher/rancher/pkg/wrangler"
	wranglerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

const (
	systemNS            = "cattle-system"
	eksAPIGroup         = "eks.cattle.io"
	eksV1               = "eks.cattle.io/v1"
	eksOperatorTemplate = "system-library-rancher-eks-operator"
	eksOperator         = "rancher-eks-operator"
	localCluster        = "local"
	importedAnno        = "eks.cattle.io/imported"
	awsAccessKey        = "amazonec2credentialConfig-accessKey"
	awsSecretKey        = "amazonec2credentialConfig-secretKey"
)

// TODO: switch to wrangler caches and clients for all types
type eksOperatorController struct {
	clusterEnqueueAfter  func(name string, duration time.Duration)
	secretsCache         wranglerv1.SecretCache
	templateCache        v3.CatalogTemplateCache
	projectCache         v3.ProjectCache
	appLister            projectv3.AppLister
	appClient            projectv3.AppInterface
	nsClient             corev1.NamespaceInterface
	clusterClient        v3.ClusterClient
	systemAccountManager *systemaccount.Manager
	dynamicClient        dynamic.NamespaceableResourceInterface
}

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext) {
	eksClusterConfigResource := schema.GroupVersionResource{
		Group:    eksAPIGroup,
		Version:  "v1",
		Resource: "eksclusterconfigs",
	}

	eksCCDynamicClient := mgmtCtx.DynamicClient.Resource(eksClusterConfigResource)
	e := &eksOperatorController{
		clusterEnqueueAfter:  wContext.Mgmt.Cluster().EnqueueAfter,
		secretsCache:         wContext.Core.Secret().Cache(),
		templateCache:        wContext.Mgmt.CatalogTemplate().Cache(),
		projectCache:         wContext.Mgmt.Project().Cache(),
		appLister:            mgmtCtx.Project.Apps("").Controller().Lister(),
		appClient:            mgmtCtx.Project.Apps(""),
		nsClient:             mgmtCtx.Core.Namespaces(""),
		clusterClient:        wContext.Mgmt.Cluster(),
		systemAccountManager: systemaccount.NewManager(mgmtCtx),
		dynamicClient:        eksCCDynamicClient,
	}

	wContext.Mgmt.Cluster().OnChange(ctx, "eks-operator-controller", e.onClusterChange)
}

func (e *eksOperatorController) onClusterChange(key string, cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	if cluster == nil || cluster.DeletionTimestamp != nil {
		return nil, nil
	}

	if cluster.Spec.EKSConfig == nil {
		return cluster, nil
	}

	if err := e.deployEKSOperator(); err != nil {
		return cluster, err
	}

	// set driver name
	if cluster.Status.Driver == "" {
		cluster = cluster.DeepCopy()
		cluster.Status.Driver = apimgmtv3.ClusterDriverEKS
		var err error
		cluster, err = e.clusterClient.Update(cluster)
		if err != nil {
			return cluster, err
		}
	}

	// get EKS Cluster Config, if it does not exist, create it
	eksClusterConfigDynamic, err := e.dynamicClient.Namespace(namespace.GlobalNamespace).Get(context.TODO(), cluster.Name, v1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return cluster, err
		}

		cluster, err = e.setUnknown(cluster, apimgmtv3.ClusterConditionWaiting, "Waiting for API to be available")
		if err != nil {
			return cluster, err
		}

		eksClusterConfigDynamic, err = buildEKSCCObject(cluster)
		if err != nil {
			return cluster, err
		}

		eksClusterConfigDynamic, err = e.dynamicClient.Namespace(namespace.GlobalNamespace).Create(context.TODO(), eksClusterConfigDynamic, v1.CreateOptions{})
		if err != nil {
			return cluster, err
		}

		cluster, err = e.recordAppliedSpec(cluster)
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
		// if cluster is imported make sure it's original spec has been copied copied before mutating
		if !cluster.Spec.EKSConfig.Imported || cluster.GetAnnotations()[importedAnno] == "true" {
			return e.updateEKSClusterConfig(cluster, eksClusterConfigDynamic, eksClusterConfigMap)
		}
	}

	// get EKS Cluster Config's phase
	status, _ := eksClusterConfigDynamic.Object["status"].(map[string]interface{})
	phase, _ := status["phase"]

	switch phase {
	case "creating":
		// set provisioning to unknown
		cluster, err = e.setUnknown(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		logrus.Infof("waiting for EKS cluster [%s] to finish creating", cluster.Name)
		e.clusterEnqueueAfter(cluster.Name, 20*time.Second)
		return cluster, nil
	case "active":
		if cluster.Spec.EKSConfig.Imported {
			// copy upstream spec if newly imported
			if cluster.Annotations == nil || cluster.Annotations[importedAnno] != "true" {
				cluster, err = e.copyImportedClusterSpec(cluster, eksClusterConfigDynamic.Object)
				if err != nil {
					return cluster, err
				}
			}
		}

		cluster, err = e.setTrue(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		// If there are no subnets it can be assumed that networking fields are not provided. In which case they
		// should be created by the eks-operator, and needs to be copied to the cluster object.
		if len(cluster.Status.EKSStatus.Subnets) == 0 {
			if status["virtualNetwork"] != "" {
				// network field have been generated and are ready to be copied
				virtualNetwork, _ := status["virtualNetwork"].(string)
				generatedSubnets, _ := status["subnets"].([]interface{})
				generatedSecurityGroups, _ := status["securityGroups"].([]interface{})
				cluster = cluster.DeepCopy()

				// change fields on status to not be generated
				cluster.Status.EKSStatus.VirtualNetwork = virtualNetwork
				for _, val := range generatedSubnets {
					cluster.Status.EKSStatus.Subnets = append(cluster.Status.EKSStatus.Subnets, val.(string))
				}
				for _, val := range generatedSecurityGroups {
					cluster.Status.EKSStatus.SecurityGroups = append(cluster.Status.EKSStatus.SecurityGroups, val.(string))
				}
				return e.clusterClient.Update(cluster)
			}
		}

		if cluster.Status.ServiceAccountToken == "" || cluster.Status.APIEndpoint == "" {
			cluster, err = e.generateAndSetServiceAccount(cluster)
		}

		// check for unauthorized error
		if failureMessage, _ := status["failureMessage"].(string); failureMessage != "" {
			if strings.Contains(failureMessage, "403") {
				return e.setFalse(cluster, apimgmtv3.ClusterConditionUpdated, "cannot access EKS, check cloud credential")
			}
		}

		return e.setTrue(cluster, apimgmtv3.ClusterConditionUpdated, "")
	case "updating":
		cluster, err = e.setTrue(cluster, apimgmtv3.ClusterConditionProvisioned, "")
		if err != nil {
			return cluster, err
		}

		failureMessage, _ := status["failureMessage"].(string)
		if failureMessage != apimgmtv3.ClusterConditionUpdated.GetMessage(cluster) {
			cluster = cluster.DeepCopy()
			apimgmtv3.ClusterConditionUpdated.Message(cluster, failureMessage)
			if failureMessage != "" {
				apimgmtv3.ClusterConditionUpdated.False(cluster)
			}
			cluster, err = e.clusterClient.Update(cluster)
			if err != nil {
				return cluster, fmt.Errorf("failure setting updating condition for cluster [%s]: %v", cluster.Name, err)
			}
		}

		e.clusterEnqueueAfter(cluster.Name, 20*time.Second)
		if failureMessage == "" {
			logrus.Infof("waiting for cluster EKS [%s] to update", cluster.Name)
			return e.setUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		}
		logrus.Infof("waiting for cluster EKS [%s] update failure to be resolved", cluster.Name)
		return e.setFalse(cluster, apimgmtv3.ClusterConditionUpdated, failureMessage)
	default:
		if cluster.Spec.EKSConfig.Imported {
			cluster, err = e.setUnknown(cluster, apimgmtv3.ClusterConditionPending, "")
			if err != nil {
				return cluster, err
			}
			logrus.Infof("waiting for cluster import [%s] to start", cluster.Name)
		} else {
			logrus.Infof("waiting for cluster create [%s] to start", cluster.Name)
		}
		e.clusterEnqueueAfter(cluster.Name, 20*time.Second)
		return cluster, nil
	}
}

func (e *eksOperatorController) updateEKSClusterConfig(cluster *mgmtv3.Cluster, eksClusterConfigDynamic *unstructured.Unstructured, oldSpec map[string]interface{}) (*mgmtv3.Cluster, error) {
	// configs are not equal, need to update EKS cluster config
	// getting resource version for watch
	list, err := e.dynamicClient.Namespace(namespace.GlobalNamespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return cluster, err
	}
	selector := fields.OneTermEqualSelector("metadata.name", cluster.Name)
	w, err := e.dynamicClient.Namespace(namespace.GlobalNamespace).Watch(context.TODO(), v1.ListOptions{ResourceVersion: list.GetResourceVersion(), FieldSelector: selector.String()})
	if err != nil {
		return cluster, err
	}
	eksClusterConfigDynamic.Object["spec"] = oldSpec
	eksClusterConfigDynamic, err = e.dynamicClient.Namespace(namespace.GlobalNamespace).Update(context.TODO(), eksClusterConfigDynamic, v1.UpdateOptions{})
	if err != nil {
		return cluster, err
	}

	// update applied spec
	cluster, err = e.recordAppliedSpec(cluster)
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
			if status["phase"] != "updating" {
				continue
			}

			// this enqueue is necessary to ensure that the controller is reentered with the updating phase
			e.clusterEnqueueAfter(cluster.Name, 10*time.Second)
			return e.setUnknown(cluster, apimgmtv3.ClusterConditionUpdated, "")
		case <-timeout.C:
			return cluster, nil
		}
	}
}
func (e *eksOperatorController) generateAndSetServiceAccount(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	// service account token and API endpoint are need to deploy cluster agent, should be able to retrieve
	// from secret
	for i := 1; i < 15; i++ {
		caSecret, err := e.secretsCache.Get(namespace.GlobalNamespace, cluster.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				return cluster, err
			}
			logrus.Infof("waiting for cluster [%s] data needed to generate service account token", cluster.Name)
			time.Sleep(20 * time.Second)
			continue
		}

		// sa token generation can be its own function
		logrus.Infof("generating service account token for cluster [%s]", cluster.Name)
		sess, err := e.startAWSSession(cluster.Spec.EKSConfig.AmazonCredentialSecret)
		if err != nil {
			return cluster, nil
		}

		endpoint := string(caSecret.Data["endpoint"])
		ca := string(caSecret.Data["ca"])
		saToken, err := generateSAToken(sess, cluster.Spec.EKSConfig.DisplayName, endpoint, ca)
		if err != nil {
			return cluster, err
		}

		cluster = cluster.DeepCopy()
		cluster.Status.APIEndpoint = endpoint
		cluster.Status.CACert = ca
		cluster.Status.ServiceAccountToken = saToken
		return e.clusterClient.Update(cluster)
	}
	return cluster, fmt.Errorf("failed waiting for cluster [%s] secret", cluster.Name)
}
func (e *eksOperatorController) copyImportedClusterSpec(cluster *mgmtv3.Cluster, eksClusterConfigMap map[string]interface{}) (*mgmtv3.Cluster, error) {
	// imported annotation is not set- need to get spec from upstream cluster
	var eksClusterConfigStructured eksv1.EKSClusterConfig
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(eksClusterConfigMap, &eksClusterConfigStructured); err != nil {
		return cluster, fmt.Errorf("failed to import cluster [%s]", cluster.Name)
	}

	// copy EKS cluster config spec to cluster and set imported annotation
	cluster = cluster.DeepCopy()
	cluster.Spec.EKSConfig = &eksClusterConfigStructured.Spec
	cluster.Status.AppliedSpec = cluster.Spec
	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string)
	}
	cluster.Annotations[importedAnno] = "true"
	if !apimgmtv3.ClusterConditionPending.IsTrue(cluster) {
		apimgmtv3.ClusterConditionPending.True(cluster)
	}
	return e.clusterClient.Update(cluster)
}

func buildEKSCCObject(cluster *mgmtv3.Cluster) (*unstructured.Unstructured, error) {
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

func (e *eksOperatorController) recordAppliedSpec(cluster *mgmtv3.Cluster) (*mgmtv3.Cluster, error) {
	cluster = cluster.DeepCopy()
	cluster.Status.AppliedSpec.EKSConfig = cluster.Spec.EKSConfig
	return e.clusterClient.Update(cluster)
}

func (e *eksOperatorController) deployEKSOperator() error {
	template, err := e.templateCache.Get(namespace.GlobalNamespace, eksOperatorTemplate)
	if err != nil {
		return err
	}

	latestTemplateVersion, err := utils.LatestAvailableTemplateVersion(template)
	if err != nil {
		return err
	}

	latestVersionID := latestTemplateVersion.ExternalID

	systemProject, err := project.GetSystemProject(localCluster, e.projectCache)
	if err != nil {
		panic(err)
	}
	systemProjectID := ref.Ref(systemProject)
	_, systemProjectName := ref.Parse(systemProjectID)

	app, err := e.appLister.Get(systemProjectName, eksOperator)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		logrus.Info("deploying EKS operator into local cluster's system project")
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
				Name:      eksOperator,
				Namespace: systemProjectName,
				Annotations: map[string]string{
					rbac.CreatorIDAnn: creator.Name,
				},
			},
			Spec: apiprojv3.AppSpec{
				Description:     "Operator for provisioning EKS clusters",
				ExternalID:      latestVersionID,
				ProjectName:     appProjectName,
				TargetNamespace: systemNS,
			},
		}

		// k3s upgrader doesn't exist yet, so it will need to be created
		if _, err = e.appClient.Create(desiredApp); err != nil {
			return err
		}
	} else {
		if app.Spec.ExternalID == latestVersionID {
			// app is up to date, no action needed
			return nil
		}
		logrus.Info("updating EKS operator in local cluster's system project")
		desiredApp := app.DeepCopy()
		desiredApp.Spec.ExternalID = latestVersionID
		// new version of k3s upgrade available, update app
		if _, err = e.appClient.Update(desiredApp); err != nil {
			return err
		}
	}

	return nil
}

func generateSAToken(sess *session.Session, clusterID, endpoint, ca string) (string, error) {
	decodedCA, err := base64.StdEncoding.DecodeString(ca)
	if err != nil {
		return "", err
	}

	generator, err := token.NewGenerator(false, false)
	if err != nil {
		return "", err
	}

	awsToken, err := generator.GetWithOptions(&token.GetTokenOptions{
		Session:   sess,
		ClusterID: clusterID,
	})
	if err != nil {
		return "", err
	}

	config := &rest.Config{
		Host: endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: decodedCA,
		},
		BearerToken: awsToken.Token,
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("error creating clientset: %v", err)
	}

	_, err = clientset.DiscoveryClient.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get Kubernetes server version: %v", err)
	}

	return util.GenerateServiceAccountToken(clientset)
}

func (e *eksOperatorController) startAWSSession(cloudCredential string) (*session.Session, error) {
	awsConfig := &aws.Config{}

	ns, id := ref.Parse(cloudCredential)
	secret, err := e.secretsCache.Get(ns, id)
	if err != nil {
		return nil, err
	}

	accessKeyBytes, _ := secret.Data[awsAccessKey]
	secretKeyBytes, _ := secret.Data[awsSecretKey]
	if accessKeyBytes == nil || secretKeyBytes == nil {
		return nil, fmt.Errorf("Invalid aws cloud credential")
	}

	accessKey := string(accessKeyBytes)
	secretKey := string(secretKeyBytes)

	awsConfig.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("error getting new aws session: %v", err)
	}
	return sess, nil
}

func (e *eksOperatorController) setUnknown(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsUnknown(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.Unknown(cluster)
	condition.Message(cluster, message)
	return e.clusterClient.Update(cluster)
}

func (e *eksOperatorController) setTrue(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsTrue(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.True(cluster)
	condition.Message(cluster, message)
	return e.clusterClient.Update(cluster)
}

func (e *eksOperatorController) setFalse(cluster *mgmtv3.Cluster, condition condition.Cond, message string) (*mgmtv3.Cluster, error) {
	if condition.IsFalse(cluster) && condition.GetMessage(cluster) == message {
		return cluster, nil
	}
	cluster = cluster.DeepCopy()
	condition.False(cluster)
	condition.Message(cluster, message)
	return e.clusterClient.Update(cluster)
}
