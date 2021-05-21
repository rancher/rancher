package provisioningcluster

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/lasso/pkg/dynamic"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/gvk"
	"github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
)

func getInfraRef(rkeCluster *rkev1.RKECluster) *corev1.ObjectReference {
	gvk, _ := gvk.Get(rkeCluster)
	infraRef := &corev1.ObjectReference{
		Name: rkeCluster.Name,
	}
	infraRef.APIVersion, infraRef.Kind = gvk.ToAPIVersionAndKind()
	return infraRef
}

func objects(cluster *rancherv1.Cluster, dynamic *dynamic.Controller, dynamicSchema mgmtcontroller.DynamicSchemaCache, secrets v1.SecretCache) (result []runtime.Object, _ error) {
	infraRef := cluster.Spec.RKEConfig.InfrastructureRef
	if infraRef == nil {
		rkeCluster := rkeCluster(cluster)
		infraRef = getInfraRef(rkeCluster)
		result = append(result, rkeCluster)
	}

	rkeControlPlane := rkeControlPlane(cluster)
	result = append(result, rkeControlPlane)

	capiCluster := capiCluster(cluster, rkeControlPlane, infraRef)
	result = append(result, capiCluster)

	machineDeployments, err := machineDeployments(cluster, capiCluster, dynamic, dynamicSchema, secrets)
	if err != nil {
		return nil, err
	}

	result = append(result, machineDeployments...)
	return result, nil
}

func pruneBySchema(kind string, data map[string]interface{}, dynamicSchema mgmtcontroller.DynamicSchemaCache) error {
	ds, err := dynamicSchema.Get(strings.ToLower(kind))
	if apierror.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	for k := range data {
		if _, ok := ds.Spec.ResourceFields[k]; !ok {
			delete(data, k)
		}
	}

	return nil
}

func toMachineTemplate(nodePoolName string, cluster *rancherv1.Cluster, nodePool rancherv1.RKENodePool,
	dynamic *dynamic.Controller, dynamicSchema mgmtcontroller.DynamicSchemaCache, secrets v1.SecretCache) (runtime.Object, error) {
	apiVersion := nodePool.NodeConfig.APIVersion
	kind := nodePool.NodeConfig.Kind
	if apiVersion == "" {
		apiVersion = "provisioning.cattle.io/v1"
	}

	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	nodeConfig, err := dynamic.Get(gvk, cluster.Namespace, nodePool.NodeConfig.Name)
	if err != nil {
		return nil, err
	}

	nodePoolData, err := data.Convert(nodeConfig.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	if err := pruneBySchema(gvk.Kind, nodePoolData, dynamicSchema); err != nil {
		return nil, err
	}

	commonData, err := convert.EncodeToMap(nodePool.RKECommonNodeConfig)
	if err != nil {
		return nil, err
	}

	nodePoolData.Set("common", commonData)
	secretName := cluster.Spec.CloudCredentialSecretName
	if nodePool.CloudCredentialSecretName != "" {
		secretName = nodePool.CloudCredentialSecretName
	}

	if secretName != "" {
		_, err := secrets.Get(cluster.Namespace, secretName)
		if err != nil {
			return nil, err
		}
		nodePoolData.SetNested(secretName, "common", "cloudCredentialSecretName")
	}

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       strings.TrimSuffix(kind, "Config") + "MachineTemplate",
			"apiVersion": "rke-node.cattle.io/v1",
			"metadata": map[string]interface{}{
				"name":      nodePoolName,
				"namespace": cluster.Namespace,
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": map[string]interface{}(nodePoolData),
				},
			},
		},
	}, nil
}

func machineDeployments(cluster *rancherv1.Cluster, capiCluster *capi.Cluster, dynamic *dynamic.Controller,
	dynamicSchema mgmtcontroller.DynamicSchemaCache, secrets v1.SecretCache) (result []runtime.Object, _ error) {
	bootstrapName := name.SafeConcatName(cluster.Name, "bootstrap", "template")

	if len(cluster.Spec.RKEConfig.NodePools) > 0 {
		result = append(result, &rkev1.RKEBootstrapTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      bootstrapName,
			},
		})
	}

	nodePoolNames := map[string]bool{}
	for _, nodePool := range cluster.Spec.RKEConfig.NodePools {
		if nodePool.Quantity != nil && *nodePool.Quantity == 0 {
			continue
		}
		if nodePool.Name == "" || nodePool.NodeConfig == nil || nodePool.NodeConfig.Name == "" || nodePool.NodeConfig.Kind == "" {
			return nil, fmt.Errorf("invalid nodePool [%s] missing name or valid config", nodePool.Name)
		}
		if !nodePool.EtcdRole &&
			!nodePool.ControlPlaneRole &&
			!nodePool.WorkerRole {
			return nil, fmt.Errorf("at least one role of etcd, control-plane or worker must be assigned to nodePool [%s]", nodePool.Name)
		}

		if nodePoolNames[nodePool.Name] {
			return nil, fmt.Errorf("duplicate nodePool name [%s] used", nodePool.Name)
		}
		nodePoolNames[nodePool.Name] = true

		var (
			nodePoolName = name.SafeConcatName(cluster.Name, "nodepool", nodePool.Name)
			infraRef     corev1.ObjectReference
		)

		if nodePool.NodeConfig.APIVersion == "" || nodePool.NodeConfig.APIVersion == "provisioning.cattle.io/v1" {
			machineTemplate, err := toMachineTemplate(nodePoolName, cluster, nodePool, dynamic, dynamicSchema, secrets)
			if err != nil {
				return nil, err
			}

			result = append(result, machineTemplate)
			infraRef = corev1.ObjectReference{
				Kind:       machineTemplate.GetObjectKind().GroupVersionKind().Kind,
				Namespace:  cluster.Namespace,
				Name:       nodePoolName,
				APIVersion: "rke-node.cattle.io/v1",
			}
		} else {
			infraRef = *nodePool.NodeConfig
		}

		machineDeployment := &capi.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   cluster.Namespace,
				Name:        nodePoolName,
				Labels:      nodePool.MachineDeploymentLabels,
				Annotations: nodePool.MachineDeploymentAnnotations,
			},
			Spec: capi.MachineDeploymentSpec{
				ClusterName: capiCluster.Name,
				Replicas:    nodePool.Quantity,
				Template: capi.MachineTemplateSpec{
					ObjectMeta: capi.ObjectMeta{
						Labels: map[string]string{
							capi.ClusterLabelName:           capiCluster.Name,
							capi.MachineDeploymentLabelName: nodePoolName,
						},
						Annotations: map[string]string{},
					},
					Spec: capi.MachineSpec{
						ClusterName: capiCluster.Name,
						Bootstrap: capi.Bootstrap{
							ConfigRef: &corev1.ObjectReference{
								Kind:       "RKEBootstrapTemplate",
								Namespace:  cluster.Namespace,
								Name:       bootstrapName,
								APIVersion: "rke.cattle.io/v1",
							},
						},
						InfrastructureRef: infraRef,
					},
				},
				Paused: nodePool.Paused,
			},
		}
		if nodePool.RollingUpdate != nil {
			machineDeployment.Spec.Strategy = &capi.MachineDeploymentStrategy{
				Type: capi.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &capi.MachineRollingUpdateDeployment{
					MaxUnavailable: nodePool.RollingUpdate.MaxUnavailable,
					MaxSurge:       nodePool.RollingUpdate.MaxSurge,
				},
			}
		}

		if nodePool.EtcdRole {
			machineDeployment.Spec.Template.Labels[planner.EtcdRoleLabel] = "true"
		}

		if nodePool.ControlPlaneRole {
			machineDeployment.Spec.Template.Labels[planner.ControlPlaneRoleLabel] = "true"
			machineDeployment.Spec.Template.Labels[capi.MachineControlPlaneLabelName] = "true"
		}

		if nodePool.WorkerRole {
			machineDeployment.Spec.Template.Labels[planner.WorkerRoleLabel] = "true"
		}

		if len(nodePool.Labels) > 0 {
			if err := assign(machineDeployment.Spec.Template.Annotations, planner.LabelsAnnotation, nodePool.Labels); err != nil {
				return nil, err
			}
		}

		if len(nodePool.Taints) > 0 {
			if err := assign(machineDeployment.Spec.Template.Annotations, planner.TaintsAnnotation, nodePool.Taints); err != nil {
				return nil, err
			}
		}

		result = append(result, machineDeployment)
	}

	return result, nil
}

func assign(labels map[string]string, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	labels[key] = string(data)
	return nil
}

func rkeCluster(cluster *rancherv1.Cluster) *rkev1.RKECluster {
	return &rkev1.RKECluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
	}
}

func rkeControlPlane(cluster *rancherv1.Cluster) *rkev1.RKEControlPlane {
	return &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
		Spec: rkev1.RKEControlPlaneSpec{
			RKEClusterSpecCommon:  cluster.Spec.RKEConfig.RKEClusterSpecCommon,
			ETCDSnapshotRestore:   cluster.Spec.RKEConfig.ETCDSnapshotRestore,
			ETCDSnapshotCreate:    cluster.Spec.RKEConfig.ETCDSnapshotCreate,
			KubernetesVersion:     cluster.Spec.KubernetesVersion,
			ManagementClusterName: cluster.Status.ClusterName,
			AgentEnvVars:          cluster.Spec.AgentEnvVars,
			ClusterName:           cluster.Name,
		},
	}
}

func capiCluster(cluster *rancherv1.Cluster, rkeControlPlane *rkev1.RKEControlPlane, infraRef *corev1.ObjectReference) *capi.Cluster {
	gvk, err := gvk.Get(rkeControlPlane)
	if err != nil {
		// this is a build issue if it happens
		panic(err)
	}

	apiVersion, kind := gvk.ToAPIVersionAndKind()

	ownerGVK := rancherv1.SchemeGroupVersion.WithKind("Cluster")
	ownerAPIVersion, _ := ownerGVK.ToAPIVersionAndKind()
	return &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         ownerAPIVersion,
					Kind:               ownerGVK.Kind,
					Name:               cluster.Name,
					UID:                cluster.UID,
					Controller:         &[]bool{true}[0],
					BlockOwnerDeletion: &[]bool{true}[0],
				},
			},
		},
		Spec: capi.ClusterSpec{
			InfrastructureRef: infraRef,
			ControlPlaneRef: &corev1.ObjectReference{
				Kind:       kind,
				Namespace:  rkeControlPlane.Namespace,
				Name:       rkeControlPlane.Name,
				APIVersion: apiVersion,
			},
		},
	}
}
