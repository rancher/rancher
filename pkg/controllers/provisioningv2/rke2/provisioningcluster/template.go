package provisioningcluster

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/rancher/lasso/pkg/dynamic"
	rancherv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/machineprovision"
	mgmtcontroller "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	v1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/gvk"
	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
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
		apiVersion = rke2.DefaultMachineConfigAPIVersion
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
			"apiVersion": rke2.RKEMachineAPIVersion,
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					rke2.MachineTemplateClonedFromGroupVersionAnn: gvk.GroupVersion().String(),
					rke2.MachineTemplateClonedFromKindAnn:         gvk.Kind,
					rke2.MachineTemplateClonedFromNameAnn:         machinePool.NodeConfig.Name,
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

		if machinePool.MachineOS == "" {
			machinePool.MachineOS = rke2.DefaultMachineOS
		}
		if machinePool.MachineDeploymentLabels == nil {
			machinePool.MachineDeploymentLabels = make(map[string]string)
		}
		machinePool.MachineDeploymentLabels[rke2.CattleOSLabel] = machinePool.MachineOS

		machineDeploymentLabels := map[string]string{}
		for k, v := range machinePool.Labels {
			machineDeploymentLabels[k] = v
		}
		for k, v := range machinePool.MachineDeploymentLabels {
			machineDeploymentLabels[k] = v
		}

		machineSpecAnnotations := map[string]string{}
		// Ignore drain if DrainBeforeDelete is unset or the pool is for etcd nodes
		if !machinePool.DrainBeforeDelete || machinePool.EtcdRole {
			machineSpecAnnotations[capi.ExcludeNodeDrainingAnnotation] = "true"
		}

		machineDeployment := &capi.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   cluster.Namespace,
				Name:        machinePoolName,
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
								APIVersion: rke2.RKEAPIVersion,
							},
						},
						InfrastructureRef: infraRef,
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
			machineDeployment.Spec.Template.Labels[rke2.EtcdRoleLabel] = "true"
		}

		if machinePool.ControlPlaneRole {
			machineDeployment.Spec.Template.Labels[rke2.ControlPlaneRoleLabel] = "true"
			machineDeployment.Spec.Template.Labels[capi.MachineControlPlaneLabelName] = "true"
		}

		if machinePool.WorkerRole {
			machineDeployment.Spec.Template.Labels[rke2.WorkerRoleLabel] = "true"
		}

		if len(machinePool.MachineOS) > 0 {
			machineDeployment.Spec.Template.Labels[rke2.CattleOSLabel] = machinePool.MachineOS
		} else {
			machineDeployment.Spec.Template.Labels[rke2.CattleOSLabel] = rke2.DefaultMachineOS
		}

		if len(machinePool.Labels) > 0 {
			for k, v := range machinePool.Labels {
				machineDeployment.Spec.Template.Labels[k] = v
			}
			if err := assign(machineDeployment.Spec.Template.Annotations, rke2.LabelsAnnotation, machinePool.Labels); err != nil {
				return nil, err
			}
		}

		if len(machinePool.Taints) > 0 {
			if err := assign(machineDeployment.Spec.Template.Annotations, rke2.TaintsAnnotation, machinePool.Taints); err != nil {
				return nil, err
			}
		}

		result = append(result, machineDeployment)

		// if a health check timeout was specified create health checks for this machine pool
		if machinePool.UnhealthyNodeTimeout != nil {
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
					capi.MachineDeploymentLabelName: machineDeployment.Name,
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

// compressInterface is a function that will marshal, gzip, then base64 encode the provided interface.
func compressInterface(v interface{}) (string, error) {
	var b64GZCluster string
	marshalledCluster, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(marshalledCluster); err != nil {
		return "", err
	}
	if err := gz.Flush(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	b64GZCluster = base64.StdEncoding.EncodeToString(b.Bytes())
	return b64GZCluster, nil
}

// decompressClusterSpec accepts an input string that is a base64/compressed cluster spec and will return a pointer to the cluster spec decompressed
func decompressClusterSpec(inputb64 string) (*rancherv1.ClusterSpec, error) {
	if inputb64 == "" {
		return nil, fmt.Errorf("empty base64 input")
	}

	decodedGzip, err := base64.StdEncoding.DecodeString(inputb64)
	if err != nil {
		return nil, fmt.Errorf("error base64.DecodeString: %v", err)
	}

	buffer := bytes.NewBuffer(decodedGzip)

	var gz io.Reader
	gz, err = gzip.NewReader(buffer)
	if err != nil {
		return nil, err
	}

	csBytes, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}

	c := rancherv1.ClusterSpec{}
	err = json.Unmarshal(csBytes, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// rkeControlPlane generates the rkecontrolplane object for a provided cluster object
func rkeControlPlane(cluster *rancherv1.Cluster) (*rkev1.RKEControlPlane, error) {
	// We need to base64/gzip encode the spec of our rancherv1.Cluster object so that we can reference it from the
	// downstream cluster
	b64GZCluster, err := compressInterface(cluster.Spec)
	if err != nil {
		logrus.Errorf("cluster: %s/%s : error while gz/b64 encoding cluster specification: %v", cluster.Namespace, cluster.ClusterName, err)
		return nil, err
	}
	return &rkev1.RKEControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				rke2.InitNodeMachineIDLabel: cluster.Labels[rke2.InitNodeMachineIDLabel],
			},
			Annotations: map[string]string{
				rke2.ClusterSpecAnnotation: b64GZCluster,
			},
		},
		Spec: rkev1.RKEControlPlaneSpec{
			RKEClusterSpecCommon:     *cluster.Spec.RKEConfig.RKEClusterSpecCommon.DeepCopy(),
			LocalClusterAuthEndpoint: *cluster.Spec.LocalClusterAuthEndpoint.DeepCopy(),
			ETCDSnapshotRestore:      cluster.Spec.RKEConfig.ETCDSnapshotRestore.DeepCopy(),
			ETCDSnapshotCreate:       cluster.Spec.RKEConfig.ETCDSnapshotCreate.DeepCopy(),
			RotateCertificates:       cluster.Spec.RKEConfig.RotateCertificates.DeepCopy(),
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
