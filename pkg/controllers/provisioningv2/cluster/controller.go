package cluster

import (
	"context"
	"regexp"
	"strconv"

	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/features"
	fleetconst "github.com/rancher/rancher/pkg/fleet"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/image"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/genericcondition"
	"github.com/rancher/wrangler/pkg/kstatus"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"github.com/rancher/wrangler/pkg/yaml"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ByCluster        = "by-cluster"
	ByCloudCred      = "by-cloud-cred"
	creatorIDAnn     = "field.cattle.io/creatorId"
	administratedAnn = "provisioning.cattle.io/administrated"
)

var (
	mgmtNameRegexp = regexp.MustCompile("^(c-[a-z0-9]{5}|local)$")
)

type handler struct {
	mgmtClusterCache      mgmtcontrollers.ClusterCache
	mgmtClusters          mgmtcontrollers.ClusterController
	clusterTokenCache     mgmtcontrollers.ClusterRegistrationTokenCache
	clusterTokens         mgmtcontrollers.ClusterRegistrationTokenClient
	featureCache          mgmtcontrollers.FeatureCache
	featureClient         mgmtcontrollers.FeatureClient
	clusters              rocontrollers.ClusterController
	clusterCache          rocontrollers.ClusterCache
	rkeControlPlanes      rkecontrollers.RKEControlPlaneClient
	rkeControlPlanesCache rkecontrollers.RKEControlPlaneCache
	secretCache           corecontrollers.SecretCache
	kubeconfigManager     *kubeconfig.Manager
	apply                 apply.Apply

	capiClustersCache capicontrollers.ClusterCache
	capiClusters      capicontrollers.ClusterClient
	capiMachinesCache capicontrollers.MachineCache
}

func Register(
	ctx context.Context,
	clients *wrangler.Context) {
	h := handler{
		mgmtClusterCache:      clients.Mgmt.Cluster().Cache(),
		mgmtClusters:          clients.Mgmt.Cluster(),
		clusterTokenCache:     clients.Mgmt.ClusterRegistrationToken().Cache(),
		clusterTokens:         clients.Mgmt.ClusterRegistrationToken(),
		featureCache:          clients.Mgmt.Feature().Cache(),
		featureClient:         clients.Mgmt.Feature(),
		clusters:              clients.Provisioning.Cluster(),
		clusterCache:          clients.Provisioning.Cluster().Cache(),
		rkeControlPlanes:      clients.RKE.RKEControlPlane(),
		rkeControlPlanesCache: clients.RKE.RKEControlPlane().Cache(),
		secretCache:           clients.Core.Secret().Cache(),
		capiClustersCache:     clients.CAPI.Cluster().Cache(),
		capiClusters:          clients.CAPI.Cluster(),
		capiMachinesCache:     clients.CAPI.Machine().Cache(),
		kubeconfigManager:     kubeconfig.New(clients),
		apply: clients.Apply.WithCacheTypes(
			clients.Provisioning.Cluster(),
			clients.Mgmt.Cluster()),
	}

	mgmtcontrollers.RegisterClusterGeneratingHandler(ctx,
		clients.Mgmt.Cluster(),
		clients.Apply.WithCacheTypes(clients.Provisioning.Cluster()),
		"",
		"provisioning-cluster-create",
		h.generateProvisioning,
		nil)

	clusterCreateApply := clients.Apply.WithCacheTypes(clients.Mgmt.Cluster(),
		clients.Mgmt.ClusterRegistrationToken(),
		clients.RBAC.ClusterRoleBinding(),
		clients.Core.Namespace(),
		clients.Core.Secret())

	if features.MCM.Enabled() {
		clusterCreateApply = clusterCreateApply.WithCacheTypes(clients.Mgmt.ClusterRoleTemplateBinding())
	}

	rocontrollers.RegisterClusterGeneratingHandler(ctx,
		clients.Provisioning.Cluster(),
		clusterCreateApply,
		"Created",
		"cluster-create",
		h.generateCluster,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		},
	)

	clients.Mgmt.Cluster().OnChange(ctx, "cluster-watch", h.createToken)
	relatedresource.Watch(ctx, "cluster-watch", h.clusterWatch,
		clients.Provisioning.Cluster(), clients.Mgmt.Cluster())

	clients.Mgmt.Cluster().OnRemove(ctx, "mgmt-cluster-remove", h.OnMgmtClusterRemove)
	clients.Provisioning.Cluster().OnRemove(ctx, "provisioning-cluster-remove", h.OnClusterRemove)
}

func RegisterIndexers(context *config.ScaledContext) {
	if features.ProvisioningV2.Enabled() {
		context.Wrangler.Provisioning.Cluster().Cache().AddIndexer(ByCluster, byClusterIndex)
		context.Wrangler.Provisioning.Cluster().Cache().AddIndexer(ByCloudCred, byCloudCredentialIndex)
	}
}

func byClusterIndex(obj *v1.Cluster) ([]string, error) {
	if obj.Status.ClusterName == "" {
		return nil, nil
	}
	return []string{obj.Status.ClusterName}, nil
}

func byCloudCredentialIndex(obj *v1.Cluster) ([]string, error) {
	if obj.Spec.CloudCredentialSecretName == "" {
		return nil, nil
	}
	return []string{obj.Spec.CloudCredentialSecretName}, nil
}

func (h *handler) clusterWatch(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	cluster, ok := obj.(*v3.Cluster)
	if !ok {
		return nil, nil
	}
	operatorClusters, err := h.clusterCache.GetByIndex(ByCluster, cluster.Name)
	if err != nil || len(operatorClusters) == 0 {
		// ignore
		return nil, nil
	}
	return []relatedresource.Key{
		{
			Namespace: operatorClusters[0].Namespace,
			Name:      operatorClusters[0].Name,
		},
	}, nil
}

func (h *handler) isLegacyCluster(cluster interface{}) bool {
	if c, ok := cluster.(*v3.Cluster); ok {
		return mgmtNameRegexp.MatchString(c.Name)
	} else if c, ok := cluster.(*v1.Cluster); ok {
		return mgmtNameRegexp.MatchString(c.Name)
	}
	return false
}

func (h *handler) generateProvisioning(cluster *v3.Cluster, status v3.ClusterStatus) ([]runtime.Object, v3.ClusterStatus, error) {
	if !h.isLegacyCluster(cluster) || cluster.Spec.FleetWorkspaceName == "" {
		return nil, status, nil
	}
	return []runtime.Object{
		&v1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        cluster.Name,
				Namespace:   cluster.Spec.FleetWorkspaceName,
				Labels:      yaml.CleanAnnotationsForExport(cluster.Labels),
				Annotations: yaml.CleanAnnotationsForExport(cluster.Annotations),
			},
		},
	}, status, nil
}

func (h *handler) generateCluster(cluster *v1.Cluster, status v1.ClusterStatus) ([]runtime.Object, v1.ClusterStatus, error) {
	switch {
	case cluster.Spec.ClusterAPIConfig != nil:
		return h.createClusterAndDeployAgent(cluster, status)
	default:
		return h.createCluster(cluster, status, v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
		})
	}
}

func NormalizeCluster(cluster *v3.Cluster, isImportedCluster bool) (runtime.Object, error) {
	// We do this so that we don't clobber status because the rancher object is pretty dirty and doesn't have a status subresource
	data, err := convert.EncodeToMap(cluster)
	if err != nil {
		return nil, err
	}
	spec, _ := data["spec"].(map[string]interface{})
	if _, ok := spec["localClusterAuthEndpoint"]; ok && isImportedCluster {
		// For imported clusters, we need to delete the localClusterAuthEndpoint so that it doesn't get overwritten here.
		// In general, imported clusters don't support localClusterAuthEndpoint.
		// However, imported RKE2/K3S clusters do and this is driven by the management cluster.
		delete(spec, "localClusterAuthEndpoint")
	}
	data = map[string]interface{}{
		"metadata": data["metadata"],
		"spec":     spec,
	}
	data["kind"] = "Cluster"
	data["apiVersion"] = "management.cattle.io/v3"
	return &unstructured.Unstructured{Object: data}, nil
}

func (h *handler) createToken(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil {
		return cluster, nil
	}
	_, err := h.clusterTokenCache.Get(cluster.Name, "default-token")
	if apierror.IsNotFound(err) {
		_, err = h.clusterTokens.Create(&v3.ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default-token",
				Namespace: cluster.Name,
			},
			Spec: v3.ClusterRegistrationTokenSpec{
				ClusterName: cluster.Name,
			},
		})
		return cluster, err
	}
	return cluster, err
}

func (h *handler) createCluster(cluster *v1.Cluster, status v1.ClusterStatus, spec v3.ClusterSpec) ([]runtime.Object, v1.ClusterStatus, error) {
	if h.isLegacyCluster(cluster) {
		mgmtCluster, err := h.mgmtClusterCache.Get(cluster.Name)
		if err != nil {
			return nil, status, err
		}
		return h.updateStatus(nil, cluster, status, mgmtCluster)
	}
	return h.createNewCluster(cluster, status, spec)
}

func mgmtClusterName() (string, error) {
	rand, err := randomtoken.Generate()
	if err != nil {
		return "", err
	}
	return name.SafeConcatName("c", "m", rand[:8]), nil
}

func (h *handler) createNewCluster(cluster *v1.Cluster, status v1.ClusterStatus, spec v3.ClusterSpec) ([]runtime.Object, v1.ClusterStatus, error) {
	spec.DisplayName = cluster.Name
	spec.Description = cluster.Annotations["field.cattle.io/description"]
	spec.FleetWorkspaceName = cluster.Namespace
	spec.DefaultPodSecurityPolicyTemplateName = cluster.Spec.DefaultPodSecurityPolicyTemplateName
	spec.DefaultClusterRoleForProjectMembers = cluster.Spec.DefaultClusterRoleForProjectMembers
	spec.EnableNetworkPolicy = cluster.Spec.EnableNetworkPolicy
	spec.DesiredAgentImage = image.ResolveWithCluster(settings.AgentImage.Get(), cluster)
	spec.DesiredAuthImage = image.ResolveWithCluster(settings.AuthImage.Get(), cluster)

	spec.ClusterSecrets.PrivateRegistrySecret = image.GetPrivateRepoSecretFromCluster(cluster)
	spec.ClusterSecrets.PrivateRegistryURL = image.GetPrivateRepoURLFromCluster(cluster)

	spec.AgentEnvVars = nil
	for _, env := range cluster.Spec.AgentEnvVars {
		spec.AgentEnvVars = append(spec.AgentEnvVars, corev1.EnvVar{
			Name:  env.Name,
			Value: env.Value,
		})
	}

	if cluster.Spec.RKEConfig != nil {
		if err := h.updateFeatureLockedValue(true); err != nil {
			return nil, status, err
		}
	}

	spec.LocalClusterAuthEndpoint = v3.LocalClusterAuthEndpoint{
		FQDN:    cluster.Spec.LocalClusterAuthEndpoint.FQDN,
		CACerts: cluster.Spec.LocalClusterAuthEndpoint.CACerts,
		Enabled: cluster.Spec.LocalClusterAuthEndpoint.Enabled,
	}

	newCluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Status.ClusterName,
			Labels:      cluster.Labels,
			Annotations: map[string]string{administratedAnn: strconv.FormatBool(cluster.Spec.RKEConfig != nil)},
		},
		Spec: spec,
	}

	if newCluster.Name == "" {
		mgmtName, err := mgmtClusterName()
		if err != nil {
			return nil, status, err
		}
		newCluster.Name = mgmtName
	}

	for k, v := range cluster.Annotations {
		newCluster.Annotations[k] = v
	}

	delete(cluster.Annotations, creatorIDAnn)

	normalizedCluster, err := NormalizeCluster(newCluster, cluster.Spec.RKEConfig == nil)
	if err != nil {
		return nil, status, err
	}

	return h.updateStatus([]runtime.Object{
		normalizedCluster,
	}, cluster, status, newCluster)
}

func (h *handler) updateStatus(objs []runtime.Object, cluster *v1.Cluster, status v1.ClusterStatus, rCluster *v3.Cluster) ([]runtime.Object, v1.ClusterStatus, error) {
	ready := false
	existing, err := h.mgmtClusterCache.Get(rCluster.Name)
	if err != nil && !apierror.IsNotFound(err) {
		return nil, status, err
	} else if err == nil {
		if condition.Cond("Ready").IsTrue(existing) {
			ready = true
		}
		for _, messageCond := range existing.Status.Conditions {
			if messageCond.Type == "Updated" || messageCond.Type == "Provisioned" || messageCond.Type == "Removed" {
				continue
			}

			found := false
			newCond := genericcondition.GenericCondition{
				Type:               string(messageCond.Type),
				Status:             messageCond.Status,
				LastUpdateTime:     messageCond.LastUpdateTime,
				LastTransitionTime: messageCond.LastTransitionTime,
				Reason:             messageCond.Reason,
				Message:            messageCond.Message,
			}
			for i, provCond := range status.Conditions {
				if provCond.Type != string(messageCond.Type) {
					continue
				}
				found = true
				status.Conditions[i] = newCond
			}
			if !found {
				status.Conditions = append(status.Conditions, newCond)
			}
		}
		status.AgentDeployed = rke2.AgentDeployed.IsTrue(existing)
	}

	// Never set ready back to false because we will end up deleting the secret
	status.Ready = status.Ready || ready
	status.ObservedGeneration = cluster.Generation
	status.ClusterName = rCluster.Name
	if status.Ready {
		kstatus.SetActive(&status)
	} else {
		kstatus.SetTransitioning(&status, "")
	}

	if status.Ready {
		secret, err := h.kubeconfigManager.GetKubeConfig(cluster, status)
		if err != nil {
			return nil, status, err
		}
		if secret != nil {
			if secret.UID == "" {
				objs = append(objs, secret)
			}
			status.ClientSecretName = secret.Name

			if features.MCM.Enabled() {
				crtb, err := h.kubeconfigManager.GetCRTBForClusterOwner(cluster, status)
				if err != nil {
					return nil, status, err
				}
				objs = append(objs, crtb)
			} else if cluster.Namespace == fleetconst.ClustersLocalNamespace && cluster.Name == "local" {
				user, err := h.kubeconfigManager.EnsureUser(cluster.Namespace, cluster.Name)
				if err != nil {
					return objs, status, err
				}
				objs = append(objs, &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fleet-local-cluster-admin",
					},
					Subjects: []rbacv1.Subject{{
						Kind:     "User",
						APIGroup: rbacv1.GroupName,
						Name:     user,
					}},
					RoleRef: rbacv1.RoleRef{
						Kind: "ClusterRole",
						Name: "cluster-admin",
					},
				})
			}
		}
	}

	return objs, status, nil
}

func (h *handler) updateFeatureLockedValue(lockValueToTrue bool) error {
	feature, err := h.featureCache.Get(features.RKE2.Name())
	if err != nil {
		return err
	}

	if feature.Status.LockedValue == nil && !lockValueToTrue || feature.Status.LockedValue != nil && *feature.Status.LockedValue == lockValueToTrue {
		return nil
	}

	feature = feature.DeepCopy()
	if lockValueToTrue {
		feature.Status.LockedValue = &lockValueToTrue
	} else {
		clusters, err := h.clusters.Cache().List("", labels.Everything())
		if err != nil {
			return err
		}

		for _, cluster := range clusters {
			if cluster.DeletionTimestamp.IsZero() && !h.isLegacyCluster(cluster) && cluster.Spec.RKEConfig != nil {
				return nil
			}
		}
		feature.Status.LockedValue = nil
	}

	_, err = h.featureClient.Update(feature)
	return err
}
