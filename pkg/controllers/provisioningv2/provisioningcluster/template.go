package provisioningcluster

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/lasso/pkg/dynamic"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	"github.com/rancher/rancher/pkg/controllers/capr/machineprovision"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/gvk"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
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

// objects generates the corresponding rkecontrolplanes.rke.cattle.io, clusters.cluster.x-k8s.io, and
// machinedeployments.cluster.x-k8s.io objects based on the passed in clusters.provisioning.cattle.io object
func objects(cluster *rancherv1.Cluster, dynamic *dynamic.Controller, dynamicSchema mgmtcontroller.DynamicSchemaCache, secrets v1.SecretCache) (result []runtime.Object, _ error) {
	if !cluster.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	infraRef := cluster.Spec.RKEConfig.InfrastructureRef
	if infraRef == nil {
		rkeCluster := rkeCluster(cluster)
		infraRef = getInfraRef(rkeCluster)
		result = append(result, rkeCluster)
	}

	rkeControlPlane, err := rkeControlPlane(cluster)
	if err != nil {
		return nil, err
	}
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

func pruneBySchema(data map[string]interface{}, dynamicSchemaSpec v3.DynamicSchemaSpec) {
	for k := range data {
		if _, ok := dynamicSchemaSpec.ResourceFields[k]; !ok {
			delete(data, k)
		}
	}
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
	dynamic *dynamic.Controller, secrets v1.SecretCache) (*unstructured.Unstructured, error) {
	apiVersion := machinePool.NodeConfig.APIVersion
	kind := machinePool.NodeConfig.Kind
	if apiVersion == "" {
		apiVersion = capr.DefaultMachineConfigAPIVersion
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

	if machinePool.DynamicSchemaSpec == "" {
		logrus.Debugf("rkecluster %s/%s: waiting for dynamic schema to be populated for machine pool %s", cluster.Namespace, cluster.Name, machinePoolName)
		return nil, generic.ErrSkip
	}
	var spec v3.DynamicSchemaSpec
	err = json.Unmarshal([]byte(machinePool.DynamicSchemaSpec), &spec)
	if err != nil {
		return nil, err
	}

	pruneBySchema(machinePoolData, spec)

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
			"apiVersion": capr.RKEMachineAPIVersion,
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					capr.MachineTemplateClonedFromGroupVersionAnn: gvk.GroupVersion().String(),
					capr.MachineTemplateClonedFromKindAnn:         gvk.Kind,
					capr.MachineTemplateClonedFromNameAnn:         machinePool.NodeConfig.Name,
				},
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
	mtHash := createMachineTemplateHash(ustr.Object)
	ustr.SetName(name.SafeConcatName(ustr.GetName(), mtHash))
	newLabels := ustr.GetLabels()
	newLabels[capr.MachineTemplateHashLabel] = mtHash
	ustr.SetLabels(newLabels)
	return ustr, nil
}

func populateHostnameLengthLimitAnnotation(mp rancherv1.RKEMachinePool, cluster *rancherv1.Cluster, annotations map[string]string) error {
	if cluster == nil {
		return errors.New("cannot add hostname length limit annotation for nil cluster")
	}
	if annotations == nil {
		return errors.Errorf("cannot add hostname length limit annotation for nil annotations")
	}
	hostnameLimit := 0

	if limit := mp.HostnameLengthLimit; limit != 0 {
		if limit < capr.MinimumHostnameLengthLimit {
			logrus.Errorf("rkecluster %s/%s: cannot use machine pool %s hostname length limit, %d under minimum value of %d", cluster.Namespace, cluster.Name, mp.Name, limit, capr.MinimumHostnameLengthLimit)
		} else if limit > capr.MaximumHostnameLengthLimit {
			logrus.Errorf("rkecluster %s/%s: cannot use machine pool %s hostname length limit, %d under minimum value of %d", cluster.Namespace, cluster.Name, mp.Name, limit, capr.MinimumHostnameLengthLimit)
		} else {
			hostnameLimit = limit
		}
	}

	// if the machine pool limit was not specified, or was invalid, fallback to cluster default
	if limit := cluster.Spec.RKEConfig.MachinePoolDefaults.HostnameLengthLimit; hostnameLimit == 0 && limit != 0 {
		if limit < capr.MinimumHostnameLengthLimit {
			logrus.Errorf("rkecluster %s/%s: cannot use cluster machine pool default hostname length limit, %d under minimum value of %d", cluster.Namespace, cluster.Name, limit, capr.MinimumHostnameLengthLimit)
		} else if limit > capr.MaximumHostnameLengthLimit {
			logrus.Errorf("rkecluster %s/%s: cannot use cluster machine pool default hostname length limit, %d under minimum value of %d", cluster.Namespace, cluster.Name, limit, capr.MinimumHostnameLengthLimit)
		} else {
			hostnameLimit = limit
		}
	}

	if hostnameLimit != 0 {
		annotations[capr.HostnameLengthLimitAnnotation] = strconv.Itoa(hostnameLimit)
	}

	return nil
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
				Labels: map[string]string{
					capr.ClusterNameLabel: cluster.Name,
				},
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
			machineDeploymentName = name.SafeConcatName(cluster.Name, machinePool.Name)
			infraRef              corev1.ObjectReference
		)

		if machinePool.NodeConfig.APIVersion == "" || machinePool.NodeConfig.APIVersion == "rke-machine-config.cattle.io/v1" {
			machineTemplate, err := toMachineTemplate(machineDeploymentName, cluster, machinePool, dynamic, secrets)
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

		if machinePool.MachineOS == "" {
			machinePool.MachineOS = capr.DefaultMachineOS
		}
		if machinePool.MachineDeploymentLabels == nil {
			machinePool.MachineDeploymentLabels = make(map[string]string)
		}
		machinePool.MachineDeploymentLabels[capr.CattleOSLabel] = machinePool.MachineOS

		machineDeploymentLabels := map[string]string{}
		for k, v := range machinePool.Labels {
			machineDeploymentLabels[k] = v
		}
		for k, v := range machinePool.MachineDeploymentLabels {
			machineDeploymentLabels[k] = v
		}

		machineSpecAnnotations := map[string]string{}
		// Ignore drain if DrainBeforeDelete is unset
		if !machinePool.DrainBeforeDelete {
			machineSpecAnnotations[capi.ExcludeNodeDrainingAnnotation] = "true"
		}

		err := populateHostnameLengthLimitAnnotation(machinePool, cluster, machineSpecAnnotations)
		if err != nil {
			return nil, err
		}

		machineDeployment := &capi.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   cluster.Namespace,
				Name:        machineDeploymentName,
				Labels:      machineDeploymentLabels,
				Annotations: machinePool.MachineDeploymentAnnotations,
			},
			Spec: capi.MachineDeploymentSpec{
				ClusterName: capiCluster.Name,
				Replicas:    machinePool.Quantity,
				Strategy: &capi.MachineDeploymentStrategy{
					// RollingUpdate is the default, so no harm in setting it here.
					Type: capi.RollingUpdateMachineDeploymentStrategyType,
					RollingUpdate: &capi.MachineRollingUpdateDeployment{
						// Delete oldest machines by default.
						DeletePolicy: &[]string{string(capi.OldestMachineSetDeletePolicy)}[0],
					},
				},
				Template: capi.MachineTemplateSpec{
					ObjectMeta: capi.ObjectMeta{
						Labels: map[string]string{
							capi.ClusterNameLabel:           capiCluster.Name,
							capr.ClusterNameLabel:           capiCluster.Name,
							capi.MachineDeploymentNameLabel: machineDeploymentName,
							capr.RKEMachinePoolNameLabel:    machinePool.Name,
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
								APIVersion: capr.RKEAPIVersion,
							},
						},
						InfrastructureRef: infraRef,
						NodeDrainTimeout:  machinePool.DrainBeforeDeleteTimeout,
					},
				},
				Paused: machinePool.Paused,
			},
		}
		if machinePool.RollingUpdate != nil {
			machineDeployment.Spec.Strategy.RollingUpdate.MaxSurge = machinePool.RollingUpdate.MaxSurge
			machineDeployment.Spec.Strategy.RollingUpdate.MaxUnavailable = machinePool.RollingUpdate.MaxUnavailable
		}

		if machinePool.EtcdRole {
			machineDeployment.Spec.Template.Labels[capr.EtcdRoleLabel] = "true"
		}

		if machinePool.ControlPlaneRole {
			machineDeployment.Spec.Template.Labels[capr.ControlPlaneRoleLabel] = "true"
			machineDeployment.Spec.Template.Labels[capi.MachineControlPlaneNameLabel] = "true"
		}

		if machinePool.WorkerRole {
			machineDeployment.Spec.Template.Labels[capr.WorkerRoleLabel] = "true"
		}

		if len(machinePool.MachineOS) > 0 {
			machineDeployment.Spec.Template.Labels[capr.CattleOSLabel] = machinePool.MachineOS
		} else {
			machineDeployment.Spec.Template.Labels[capr.CattleOSLabel] = capr.DefaultMachineOS
		}

		if len(machinePool.Labels) > 0 {
			for k, v := range machinePool.Labels {
				machineDeployment.Spec.Template.Labels[k] = v
			}
			if err := assign(machineDeployment.Spec.Template.Annotations, capr.LabelsAnnotation, machinePool.Labels); err != nil {
				return nil, err
			}
		}

		if len(machinePool.Taints) > 0 {
			if err := assign(machineDeployment.Spec.Template.Annotations, capr.TaintsAnnotation, machinePool.Taints); err != nil {
				return nil, err
			}
		}

		result = append(result, machineDeployment)

		// if a health check timeout was specified create health checks for this machine pool
		if machinePool.UnhealthyNodeTimeout != nil && machinePool.UnhealthyNodeTimeout.Duration > 0 {
			hc := deploymentHealthChecks(machineDeployment, machinePool)
			result = append(result, hc)
		}
	}

	return result, nil
}

// deploymentHealthChecks Health checks will mark a machine as failed if it has any of the conditions below for the duration of the given timeout. https://cluster-api.sigs.k8s.io/tasks/healthcheck.html#what-is-a-machinehealthcheck
func deploymentHealthChecks(machineDeployment *capi.MachineDeployment, machinePool rancherv1.RKEMachinePool) *capi.MachineHealthCheck {
	var maxUnhealthy *intstr.IntOrString
	if machinePool.MaxUnhealthy != nil {
		maxUnhealthy = new(intstr.IntOrString)
		*maxUnhealthy = intstr.Parse(*machinePool.MaxUnhealthy)
	}

	return &capi.MachineHealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineDeployment.Name,
			Namespace: machineDeployment.Namespace,
		},
		Spec: capi.MachineHealthCheckSpec{
			ClusterName: machineDeployment.Spec.ClusterName,
			Selector: metav1.LabelSelector{ // this health check only applies to machines in this deployment
				MatchLabels: map[string]string{
					capi.MachineDeploymentNameLabel: machineDeployment.Name,
				},
			},
			UnhealthyConditions: []capi.UnhealthyCondition{ // if a node status is unready or unknown for the timeout mark it unhealthy
				{
					Status:  corev1.ConditionUnknown,
					Type:    corev1.NodeReady,
					Timeout: *machinePool.UnhealthyNodeTimeout,
				},
				{
					Status:  corev1.ConditionFalse,
					Type:    corev1.NodeReady,
					Timeout: *machinePool.UnhealthyNodeTimeout,
				},
			},
			MaxUnhealthy:       maxUnhealthy,
			UnhealthyRange:     machinePool.UnhealthyRange,
			NodeStartupTimeout: machinePool.NodeStartupTimeout,
		},
	}
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

// rkeControlPlane generates the rkecontrolplane object for a provided cluster object
func rkeControlPlane(cluster *rancherv1.Cluster) (*rkev1.RKEControlPlane, error) {
	// We need to base64/gzip encode the spec of our rancherv1.Cluster object so that we can reference it from the
	// downstream cluster
	filteredClusterSpec := cluster.Spec.DeepCopy()
	// set the corresponding specification for various operations to nil as these cause unnecessary reconciliation.
	filteredClusterSpec.RKEConfig.ETCDSnapshotRestore = nil
	filteredClusterSpec.RKEConfig.ETCDSnapshotCreate = nil
	filteredClusterSpec.RKEConfig.RotateEncryptionKeys = nil
	filteredClusterSpec.RKEConfig.RotateCertificates = nil
	b64GZCluster, err := capr.CompressInterface(filteredClusterSpec)
	if err != nil {
		logrus.Errorf("cluster: %s/%s : error while gz/b64 encoding cluster specification: %v", cluster.Namespace, cluster.Name, err)
		return nil, err
	}
	rkeConfig := cluster.Spec.RKEConfig.DeepCopy()
	return &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				capr.InitNodeMachineIDLabel: cluster.Labels[capr.InitNodeMachineIDLabel],
			},
			Annotations: map[string]string{
				capr.ClusterSpecAnnotation: b64GZCluster,
			},
		},
		Spec: rkev1.RKEControlPlaneSpec{
			RKEClusterSpecCommon:     rkeConfig.RKEClusterSpecCommon,
			LocalClusterAuthEndpoint: *cluster.Spec.LocalClusterAuthEndpoint.DeepCopy(),
			ETCDSnapshotRestore:      rkeConfig.ETCDSnapshotRestore,
			ETCDSnapshotCreate:       rkeConfig.ETCDSnapshotCreate,
			RotateCertificates:       rkeConfig.RotateCertificates,
			RotateEncryptionKeys:     rkeConfig.RotateEncryptionKeys,
			KubernetesVersion:        cluster.Spec.KubernetesVersion,
			ManagementClusterName:    cluster.Status.ClusterName, // management cluster
			AgentEnvVars:             cluster.Spec.AgentEnvVars,
			ClusterName:              cluster.Name, // cluster name is for the CAPI cluster
		},
	}, nil
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
