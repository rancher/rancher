package cluster

import (
	"context"
	"errors"

	"github.com/rancher/norman/types/convert"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	rocontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kstatus"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/rancher/wrangler/pkg/relatedresource"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	byCluster         = "by-cluster"
	creatorIDAnn      = "field.cattle.io/creatorId"
	managedAnnotation = "provisioning.cattle.io/managed"
)

type handler struct {
	mgmtClusterCache  mgmtcontrollers.ClusterCache
	mgmtClusters      mgmtcontrollers.ClusterClient
	clusterTokenCache mgmtcontrollers.ClusterRegistrationTokenCache
	clusterTokens     mgmtcontrollers.ClusterRegistrationTokenClient
	clusters          rocontrollers.ClusterController
	clusterCache      rocontrollers.ClusterCache
	secretCache       corecontrollers.SecretCache
	settings          mgmtcontrollers.SettingCache
	kubeconfigManager *kubeconfig.Manager
	apply             apply.Apply
}

func Register(
	ctx context.Context,
	clients *wrangler.Context) {
	h := handler{
		mgmtClusterCache:  clients.Mgmt.Cluster().Cache(),
		mgmtClusters:      clients.Mgmt.Cluster(),
		clusterTokenCache: clients.Mgmt.ClusterRegistrationToken().Cache(),
		clusterTokens:     clients.Mgmt.ClusterRegistrationToken(),
		settings:          clients.Mgmt.Setting().Cache(),
		clusters:          clients.Provisioning.Cluster(),
		clusterCache:      clients.Provisioning.Cluster().Cache(),
		secretCache:       clients.Core.Secret().Cache(),
		kubeconfigManager: kubeconfig.New(clients),
		apply: clients.Apply.WithCacheTypes(
			clients.Provisioning.Cluster(),
			clients.Mgmt.Cluster()),
	}

	clients.Provisioning.Cluster().OnChange(ctx, "cluster-label", h.addLabel)
	rocontrollers.RegisterClusterGeneratingHandler(ctx,
		clients.Provisioning.Cluster(),
		clients.Apply.WithCacheTypes(clients.Mgmt.Cluster(),
			clients.Mgmt.ClusterRoleTemplateBinding(),
			clients.Mgmt.ClusterRegistrationToken(),
			clients.Core.Namespace(),
			clients.Core.Secret()),
		"Created",
		"cluster-create",
		h.generateCluster,
		&generic.GeneratingHandlerOptions{
			AllowClusterScoped: true,
		},
	)
	clients.Mgmt.Cluster().OnChange(ctx, "cluster-watch", h.createToken)
	clusterCache := clients.Provisioning.Cluster().Cache()
	relatedresource.Watch(ctx, "cluster-watch", h.clusterWatch,
		clients.Provisioning.Cluster(), clients.Mgmt.Cluster())

	clusterCache.AddIndexer(byCluster, byClusterIndex)
	clients.Provisioning.Cluster().Informer().GetIndexer().ListKeys()
}

func byClusterIndex(obj *v1.Cluster) ([]string, error) {
	if obj.Status.ClusterName == "" {
		return nil, nil
	}
	return []string{obj.Status.ClusterName}, nil
}

func (h *handler) addLabel(_ string, cluster *v1.Cluster) (*v1.Cluster, error) {
	if cluster == nil {
		return nil, nil
	}
	if cluster.Labels["metadata.name"] == cluster.Name {
		return cluster, nil
	}

	cluster = cluster.DeepCopy()
	if cluster.Labels == nil {
		cluster.Labels = map[string]string{}
	}
	cluster.Labels["metadata.name"] = cluster.Name
	return h.clusters.Update(cluster)
}

func (h *handler) clusterWatch(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	cluster, ok := obj.(*v3.Cluster)
	if !ok {
		return nil, nil
	}
	operatorClusters, err := h.clusterCache.GetByIndex(byCluster, cluster.Name)
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

func (h *handler) generateCluster(cluster *v1.Cluster, status v1.ClusterStatus) ([]runtime.Object, v1.ClusterStatus, error) {
	if cluster.Spec.ReferencedConfig != nil {
		return h.referenceCluster(cluster, status)
	}

	if owningCluster, err := h.apply.FindOwner(cluster); errors.Is(err, apply.ErrOwnerNotFound) || errors.Is(err, apply.ErrNoInformerFound) {
	} else if _, ok := owningCluster.(*v3.Cluster); err == nil && ok {
		// Do not generate v3.Cluster if this cluster was generated from a v3.Cluster
		return nil, status, nil
	}

	switch {
	case cluster.Spec.ClusterAPIConfig != nil:
		return h.createClusterAndDeployAgent(cluster, status)
	default:
		return h.createCluster(cluster, status, v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
		})
	}
}

func NormalizeCluster(cluster *v3.Cluster) (runtime.Object, error) {
	// We do this so that we don't clobber status because the rancher object is pretty dirty and doesn't have a status subresource
	data, err := convert.EncodeToMap(cluster)
	if err != nil {
		return nil, err
	}
	data = map[string]interface{}{
		"metadata": data["metadata"],
		"spec":     data["spec"],
	}
	data["kind"] = "Cluster"
	data["apiVersion"] = "management.cattle.io/v3"
	return &unstructured.Unstructured{Object: data}, nil
}

func (h *handler) createToken(_ string, cluster *v3.Cluster) (*v3.Cluster, error) {
	if cluster == nil || cluster.Annotations[managedAnnotation] != "true" {
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
	spec.DisplayName = cluster.Name
	spec.Description = cluster.Annotations["field.cattle.io/description"]
	spec.FleetWorkspaceName = cluster.Namespace
	spec.AgentEnvVars = cluster.Spec.AgentEnvVars
	spec.DefaultPodSecurityPolicyTemplateName = cluster.Spec.DefaultPodSecurityPolicyTemplateName
	spec.DefaultClusterRoleForProjectMembers = cluster.Spec.DefaultClusterRoleForProjectMembers
	spec.EnableNetworkPolicy = cluster.Spec.EnableNetworkPolicy

	if cluster.Spec.RKEConfig != nil {
		spec.LocalClusterAuthEndpoint = v3.LocalClusterAuthEndpoint{
			FQDN:    cluster.Spec.RKEConfig.LocalClusterAuthEndpoint.FQDN,
			CACerts: cluster.Spec.RKEConfig.LocalClusterAuthEndpoint.CACerts,
		}
		if cluster.Spec.RKEConfig.LocalClusterAuthEndpoint.Enabled == nil {
			spec.LocalClusterAuthEndpoint.Enabled = true
		} else {
			spec.LocalClusterAuthEndpoint.Enabled = *cluster.Spec.RKEConfig.LocalClusterAuthEndpoint.Enabled
		}
	}

	newCluster := &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name.SafeConcatName("c", "m", string(cluster.UID[:8])),
			Labels:      cluster.Labels,
			Annotations: map[string]string{},
		},
		Spec: spec,
	}

	for k, v := range cluster.Annotations {
		newCluster.Annotations[k] = v
	}

	userName, err := h.kubeconfigManager.EnsureUser(cluster.Namespace, cluster.Name)
	if err != nil {
		return nil, status, err
	}

	newCluster.Annotations[creatorIDAnn] = userName
	newCluster.Annotations[managedAnnotation] = "true"

	normalizedCluster, err := NormalizeCluster(newCluster)
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
	}

	// Never set ready back to false because we will end up deleting the secret
	status.Ready = status.Ready || ready
	status.ObservedGeneration = cluster.Generation
	status.ClusterName = rCluster.Name
	if ready {
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
			objs = append(objs, secret)
			status.ClientSecretName = secret.Name

			ctrb, err := h.kubeconfigManager.GetCTRBForAdmin(cluster, status)
			if err != nil {
				return nil, status, err
			}
			objs = append(objs, ctrb)
		}
	}

	return objs, status, nil
}
