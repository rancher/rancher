package provisioningcluster

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rancher/lasso/pkg/dynamic"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/machineprovision"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/gvk"
	"github.com/rancher/wrangler/pkg/name"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
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

func takeOwnership(dynamic *dynamic.Controller, cluster *rancherv1.Cluster, nodeConfig runtime.Object) error {
	m, err := meta.Accessor(nodeConfig)
	if err != nil {
		return err
	}

	for _, owner := range m.GetOwnerReferences() {
		if owner.Kind == "Cluster" && owner.APIVersion == "provisioning.cattle.io/v1" {
			if owner.Name != cluster.Name {
				return fmt.Errorf("can not use %s/%s [%v] because it is already owned by cluster %s",
					m.GetNamespace(), m.GetName(), nodeConfig.GetObjectKind().GroupVersionKind(), owner.Name)
			}
			return nil
		}
		if owner.Controller != nil && *owner.Controller {
			return fmt.Errorf("can not use %s/%s [%v] because it is already owned by %s %s",
				m.GetNamespace(), m.GetName(), nodeConfig.GetObjectKind().GroupVersionKind(), owner.Kind, owner.Name)
		}
	}

	// Take ownership

	nodeConfig = nodeConfig.DeepCopyObject()
	m, err = meta.Accessor(nodeConfig)
	if err != nil {
		return err
	}
	m.SetOwnerReferences(append(m.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion:         "provisioning.cattle.io/v1",
		Kind:               "Cluster",
		Name:               cluster.Name,
		UID:                cluster.UID,
		Controller:         &[]bool{true}[0],
		BlockOwnerDeletion: &[]bool{true}[0],
	}))
	_, err = dynamic.Update(nodeConfig)
	return err
}

func toMachineTemplate(machinePoolName string, cluster *rancherv1.Cluster, machinePool rancherv1.RKEMachinePool,
	dynamic *dynamic.Controller, dynamicSchema mgmtcontroller.DynamicSchemaCache, secrets v1.SecretCache) (*unstructured.Unstructured, error) {
	apiVersion := machinePool.NodeConfig.APIVersion
	kind := machinePool.NodeConfig.Kind
	if apiVersion == "" {
		apiVersion = defaultMachineConfigAPIVersion
	}

	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	nodeConfig, err := dynamic.Get(gvk, cluster.Namespace, machinePool.NodeConfig.Name)
	if err != nil {
		return nil, err
	}

	if err := takeOwnership(dynamic, cluster, nodeConfig); err != nil {
		return nil, err
	}

	machinePoolData, err := data.Convert(nodeConfig.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	if err := pruneBySchema(gvk.Kind, machinePoolData, dynamicSchema); err != nil {
		return nil, err
	}

	commonData, err := convert.EncodeToMap(machinePool.RKECommonNodeConfig)
	if err != nil {
		return nil, err
	}

	machinePoolData.Set("common", commonData)
	secretName := cluster.Spec.CloudCredentialSecretName
	if machinePool.CloudCredentialSecretName != "" {
		secretName = machinePool.CloudCredentialSecretName
	}

	if secretName != "" {
		_, err := machineprovision.GetCloudCredentialSecret(secrets, cluster.Namespace, secretName)
		if err != nil {
			return nil, err
		}
		machinePoolData.SetNested(secretName, "common", "cloudCredentialSecretName")
	}

	ustr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       strings.TrimSuffix(kind, "Config") + "MachineTemplate",
			"apiVersion": "rke-machine.cattle.io/v1",
			"metadata": map[string]interface{}{
				"name":      machinePoolName,
				"namespace": cluster.Namespace,
				"labels": map[string]interface{}{
					apply.LabelPrune: "false",
				},
			},
			"spec": map[string]interface{}{
				"clusterName": cluster.Name,
				"template": map[string]interface{}{
					"spec": map[string]interface{}(machinePoolData),
				},
			},
		},
	}
	ustr.SetName(name.SafeConcatName(ustr.GetName(), createMachineTemplateHash(ustr.Object)))
	return ustr, nil
}

func createMachineTemplateHash(dataMap map[string]interface{}) string {
	ustr := &unstructured.Unstructured{Object: dataMap}
	dataMap = ustr.DeepCopy().Object

	name, _, _ := unstructured.NestedString(dataMap, "metadata", "name")
	spec, _, _ := unstructured.NestedMap(dataMap, "spec", "template", "spec")
	unstructured.RemoveNestedField(spec, "common")
	planner.PruneEmpty(spec)

	sha := sha256.New()
	sha.Write([]byte(name))

	// ignore errors, shouldn't happen
	bytes, _ := json.Marshal(spec)

	sha.Write(bytes)

	hash := sha.Sum(nil)
	return hex.EncodeToString(hash[:])[:8]
}

func machineDeployments(cluster *rancherv1.Cluster, capiCluster *capi.Cluster, dynamic *dynamic.Controller,
	dynamicSchema mgmtcontroller.DynamicSchemaCache, secrets v1.SecretCache) (result []runtime.Object, _ error) {
	bootstrapName := name.SafeConcatName(cluster.Name, "bootstrap", "template")

	if dynamicSchema == nil {
		return nil, nil
	}

	if len(cluster.Spec.RKEConfig.MachinePools) > 0 {
		result = append(result, &rkev1.RKEBootstrapTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Namespace,
				Name:      bootstrapName,
			},
			Spec: rkev1.RKEBootstrapTemplateSpec{
				ClusterName: cluster.Name,
				Template: rkev1.RKEBootstrap{
					Spec: rkev1.RKEBootstrapSpec{
						ClusterName: cluster.Name,
					},
				},
			},
		})
	}

	machinePoolNames := map[string]bool{}
	for _, machinePool := range cluster.Spec.RKEConfig.MachinePools {
		if machinePool.Quantity != nil && *machinePool.Quantity == 0 {
			continue
		}
		if machinePool.Name == "" || machinePool.NodeConfig == nil || machinePool.NodeConfig.Name == "" || machinePool.NodeConfig.Kind == "" {
			return nil, fmt.Errorf("invalid machinePool [%s] missing name or valid config", machinePool.Name)
		}
		if !machinePool.EtcdRole &&
			!machinePool.ControlPlaneRole &&
			!machinePool.WorkerRole {
			return nil, fmt.Errorf("at least one role of etcd, control-plane or worker must be assigned to machinePool [%s]", machinePool.Name)
		}

		if machinePoolNames[machinePool.Name] {
			return nil, fmt.Errorf("duplicate machinePool name [%s] used", machinePool.Name)
		}
		machinePoolNames[machinePool.Name] = true

		var (
			machinePoolName = name.SafeConcatName(cluster.Name, machinePool.Name)
			infraRef        corev1.ObjectReference
		)

		if machinePool.NodeConfig.APIVersion == "" || machinePool.NodeConfig.APIVersion == "rke-machine-config.cattle.io/v1" {
			machineTemplate, err := toMachineTemplate(machinePoolName, cluster, machinePool, dynamic, dynamicSchema, secrets)
			if err != nil {
				return nil, err
			}

			result = append(result, machineTemplate)
			infraRef = corev1.ObjectReference{
				APIVersion: machineTemplate.GetAPIVersion(),
				Kind:       machineTemplate.GetKind(),
				Namespace:  machineTemplate.GetNamespace(),
				Name:       machineTemplate.GetName(),
			}
		} else {
			infraRef = *machinePool.NodeConfig
		}

		machineDeploymentLabels := map[string]string{}
		for k, v := range machinePool.Labels {
			machineDeploymentLabels[k] = v
		}
		for k, v := range machinePool.MachineDeploymentLabels {
			machineDeploymentLabels[k] = v
		}
		machineSpecAnnotations := map[string]string{}
		if !machinePool.DrainBeforeDelete {
			machineSpecAnnotations[capi.ExcludeNodeDrainingAnnotation] = "true"
		}

		machineDeployment := &capi.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   cluster.Namespace,
				Name:        machinePoolName,
				Labels:      machineDeploymentLabels,
				Annotations: machinePool.MachineDeploymentAnnotations,
				// This ensures that the CAPI cluster is not deleted before the machines are deleted.
				Finalizers: []string{metav1.FinalizerDeleteDependents},
			},
			Spec: capi.MachineDeploymentSpec{
				ClusterName: capiCluster.Name,
				Replicas:    machinePool.Quantity,
				Template: capi.MachineTemplateSpec{
					ObjectMeta: capi.ObjectMeta{
						Labels: map[string]string{
							capi.ClusterLabelName:           capiCluster.Name,
							capi.MachineDeploymentLabelName: machinePoolName,
						},
						Annotations: machineSpecAnnotations,
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
				Paused: machinePool.Paused,
			},
		}
		if machinePool.RollingUpdate != nil {
			machineDeployment.Spec.Strategy = &capi.MachineDeploymentStrategy{
				Type: capi.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &capi.MachineRollingUpdateDeployment{
					MaxUnavailable: machinePool.RollingUpdate.MaxUnavailable,
					MaxSurge:       machinePool.RollingUpdate.MaxSurge,
				},
			}
		}

		if machinePool.EtcdRole {
			machineDeployment.Spec.Template.Labels[planner.EtcdRoleLabel] = "true"
		}

		if machinePool.ControlPlaneRole {
			machineDeployment.Spec.Template.Labels[planner.ControlPlaneRoleLabel] = "true"
			machineDeployment.Spec.Template.Labels[capi.MachineControlPlaneLabelName] = "true"
		}

		if machinePool.WorkerRole {
			machineDeployment.Spec.Template.Labels[planner.WorkerRoleLabel] = "true"
		}

		if len(machinePool.Labels) > 0 {
			for k, v := range machinePool.Labels {
				machineDeployment.Spec.Template.Labels[k] = v
			}
			if err := assign(machineDeployment.Spec.Template.Annotations, planner.LabelsAnnotation, machinePool.Labels); err != nil {
				return nil, err
			}
		}

		if len(machinePool.Taints) > 0 {
			if err := assign(machineDeployment.Spec.Template.Annotations, planner.TaintsAnnotation, machinePool.Taints); err != nil {
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
			Labels: map[string]string{
				planner.InitNodeMachineIDLabel: cluster.Labels[planner.InitNodeMachineIDLabel],
			},
		},
		Spec: rkev1.RKEControlPlaneSpec{
			RKEClusterSpecCommon:     *cluster.Spec.RKEConfig.RKEClusterSpecCommon.DeepCopy(),
			LocalClusterAuthEndpoint: *cluster.Spec.LocalClusterAuthEndpoint.DeepCopy(),
			ETCDSnapshotRestore:      cluster.Spec.RKEConfig.ETCDSnapshotRestore.DeepCopy(),
			ETCDSnapshotCreate:       cluster.Spec.RKEConfig.ETCDSnapshotCreate.DeepCopy(),
			KubernetesVersion:        cluster.Spec.KubernetesVersion,
			ManagementClusterName:    cluster.Status.ClusterName,
			AgentEnvVars:             cluster.Spec.AgentEnvVars,
			ClusterName:              cluster.Name,
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
