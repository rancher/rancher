package machineprovision

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/management/drivers/nodedriver"
	"github.com/rancher/rancher/pkg/controllers/management/node"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"
	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/etcdmgmt"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	batchcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/genericcondition"
	"github.com/rancher/wrangler/pkg/summary"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

const (
	createJobConditionType = "CreateJob"
	deleteJobConditionType = "DeleteJob"

	forceRemoveMachineAnn = "provisioning.cattle.io/force-machine-remove"

	drainingSucceededCondition = condition.Cond(capi.DrainingSucceededCondition)
)

type infraObject struct {
	data     data.Object
	obj      runtime.Object
	meta     metav1.Object
	typeMeta metav1.Type
}

func newInfraObject(obj runtime.Object) (*infraObject, error) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	data, err := data.Convert(obj.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	typeMeta, err := meta.TypeAccessor(obj)
	if err != nil {
		return nil, err
	}

	return &infraObject{
		data:     data,
		obj:      obj,
		meta:     objMeta,
		typeMeta: typeMeta,
	}, nil
}

type handler struct {
	ctx                 context.Context
	apply               apply.Apply
	jobController       batchcontrollers.JobController
	jobs                batchcontrollers.JobCache
	pods                corecontrollers.PodCache
	secrets             corecontrollers.SecretCache
	machines            capicontrollers.MachineCache
	namespaces          corecontrollers.NamespaceCache
	nodeDriverCache     mgmtcontrollers.NodeDriverCache
	dynamic             *dynamic.Controller
	rancherClusterCache ranchercontrollers.ClusterCache
	kubeconfigManager   *kubeconfig.Manager
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		ctx: ctx,
		apply: clients.Apply.WithCacheTypes(clients.Core.Secret(),
			clients.Core.ServiceAccount(),
			clients.RBAC.RoleBinding(),
			clients.RBAC.Role(),
			clients.Batch.Job()),
		pods:                clients.Core.Pod().Cache(),
		jobController:       clients.Batch.Job(),
		jobs:                clients.Batch.Job().Cache(),
		secrets:             clients.Core.Secret().Cache(),
		machines:            clients.CAPI.Machine().Cache(),
		nodeDriverCache:     clients.Mgmt.NodeDriver().Cache(),
		namespaces:          clients.Core.Namespace().Cache(),
		dynamic:             clients.Dynamic,
		rancherClusterCache: clients.Provisioning.Cluster().Cache(),
		kubeconfigManager:   kubeconfig.New(clients),
	}

	removeHandler := generic.NewRemoveHandler("machine-provision-remove", clients.Dynamic.Update, h.OnRemove)

	clients.Dynamic.OnChange(ctx, "machine-provision-remove", validGVK, dynamic.FromKeyHandler(removeHandler))
	clients.Dynamic.OnChange(ctx, "machine-provision", validGVK, h.OnChange)
	clients.Batch.Job().OnChange(ctx, "machine-provision-pod", h.OnJobChange)
}

func validGVK(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "rke-machine.cattle.io" &&
		gvk.Version == "v1" &&
		strings.HasSuffix(gvk.Kind, "Machine") &&
		gvk.Kind != "CustomMachine"
}

func (h *handler) OnJobChange(key string, job *batchv1.Job) (*batchv1.Job, error) {
	if job == nil {
		return nil, nil
	}

	name := job.Spec.Template.Labels[InfraMachineName]
	group := job.Spec.Template.Labels[InfraMachineGroup]
	version := job.Spec.Template.Labels[InfraMachineVersion]
	kind := job.Spec.Template.Labels[InfraMachineKind]

	if name == "" || kind == "" {
		return job, nil
	}

	infraMachine, err := h.dynamic.Get(schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}, job.Namespace, name)
	if apierrors.IsNotFound(err) {
		// ignore err
		return job, nil
	} else if err != nil {
		return job, err
	}

	meta, err := meta.Accessor(infraMachine)
	if err != nil {
		return nil, err
	}

	d, err := data.Convert(infraMachine)
	if err != nil {
		return job, err
	}

	newStatus, err := h.getMachineStatus(job, job.Spec.Template.Labels[InfraJobRemove] == "true")
	if err != nil {
		return job, err
	}
	newStatus.JobName = job.Name

	if _, err := h.patchStatus(infraMachine, d, newStatus); err != nil {
		return job, err
	}

	// Re-evaluate the infra-machine after this
	if err := h.dynamic.Enqueue(infraMachine.GetObjectKind().GroupVersionKind(),
		meta.GetNamespace(), meta.GetName()); err != nil {
		return nil, err
	}

	return job, nil
}

func (h *handler) getMachineStatus(job *batchv1.Job, remove bool) (rkev1.RKEMachineStatus, error) {
	condType := createJobConditionType
	if remove {
		condType = deleteJobConditionType
	}
	if !job.Status.CompletionTime.IsZero() {
		return rkev1.RKEMachineStatus{
			Conditions: []genericcondition.GenericCondition{
				{
					Type:   condType,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
			},
			JobComplete: true,
		}, nil
	}

	if condition.Cond("Failed").IsTrue(job) {
		sel, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
		if err != nil {
			return rkev1.RKEMachineStatus{}, err
		}

		pods, err := h.pods.List(job.Namespace, sel)
		if err != nil {
			return rkev1.RKEMachineStatus{}, err
		}

		var lastPod *corev1.Pod
		for _, pod := range pods {
			if lastPod == nil {
				lastPod = pod
				continue
			} else if pod.CreationTimestamp.After(lastPod.CreationTimestamp.Time) {
				lastPod = pod
			}
		}

		if lastPod != nil {
			return getMachineStatusFromPod(lastPod, job.Spec.Template.Labels[InfraMachineKind], condType), nil
		}
	}

	message := CreatingMachineMessage(job.Spec.Template.Labels[InfraMachineKind])
	if condType != createJobConditionType {
		message = DeletingMachineMessage(job.Spec.Template.Labels[InfraMachineKind])
	}

	return rkev1.RKEMachineStatus{Conditions: []genericcondition.GenericCondition{
		{
			Type:    "Ready",
			Status:  corev1.ConditionFalse,
			Message: message,
		},
	}}, nil
}

func getMachineStatusFromPod(pod *corev1.Pod, kind, condType string) rkev1.RKEMachineStatus {
	reason := string(capierrors.CreateMachineError)
	if condType == deleteJobConditionType {
		reason = string(capierrors.DeleteMachineError)
	}

	if pod.Status.Phase == corev1.PodSucceeded {
		return rkev1.RKEMachineStatus{
			Conditions: []genericcondition.GenericCondition{
				{
					Type:   condType,
					Status: corev1.ConditionTrue,
				},
				{
					Type:   "Ready",
					Status: corev1.ConditionTrue,
				},
			},
			JobComplete: true,
		}
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
			failureMessage := strings.TrimSpace(containerStatus.State.Terminated.Message)
			message := FailedMachineCreateMessage(kind, reason, failureMessage)
			if condType != createJobConditionType {
				message = FailedMachineDeleteMessage(kind, reason, failureMessage)
			}
			return rkev1.RKEMachineStatus{
				Conditions: []genericcondition.GenericCondition{
					{
						Type:    condType,
						Status:  corev1.ConditionFalse,
						Reason:  reason,
						Message: message,
					},
					{
						Type:    "Ready",
						Status:  corev1.ConditionFalse,
						Reason:  reason,
						Message: message,
					},
				},
				FailureReason:  reason,
				FailureMessage: failureMessage,
			}
		}
	}

	return rkev1.RKEMachineStatus{}
}

func (h *handler) namespaceIsRemoved(obj runtime.Object) (bool, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}

	ns, err := h.namespaces.Get(meta.GetNamespace())
	if err != nil {
		return false, err
	}

	return ns.DeletionTimestamp != nil, nil
}

func (h *handler) OnRemove(key string, obj runtime.Object) (runtime.Object, error) {
	if removed, err := h.namespaceIsRemoved(obj); err != nil || removed {
		return obj, err
	}

	infraObj, err := newInfraObject(obj)
	if err != nil {
		return obj, err
	}

	if !infraObj.data.Bool("status", "jobComplete") && infraObj.data.String("status", "failureReason") == "" {
		return obj, fmt.Errorf("cannot delete machine %s because create job has not finished", infraObj.meta.GetName())
	}

	if cond := getCondition(infraObj.data, deleteJobConditionType); cond != nil {
		job, err := h.getJobFromInfraMachine(infraObj)
		if apierrors.IsNotFound(err) {
			// If the deletion job condition has been set on the infrastructure object and the deletion job has been removed,
			// then we don't want to create another deletion job.
			logrus.Infof("Machine %s %s has already been deleted", infraObj.obj.GetObjectKind().GroupVersionKind(), infraObj.meta.GetName())
			return obj, nil
		} else if err != nil {
			return obj, err
		}

		if shouldCleanupObjects(job, infraObj.data) {
			// Calling WithOwner(obj).ApplyObjects with no objects here will look for all objects with types passed to
			// WithCacheTypes above that have an owner label (not owner reference) to the given obj. It will compare the existing
			// objects it finds to the ones that are passed to ApplyObjects (which there are none in this case). The apply
			// controller will delete all existing objects it finds that are not passed to ApplyObjects. Since no objects are
			// passed here, it will delete all objects it finds.
			return obj, h.apply.WithOwner(obj).ApplyObjects()
		}
		return obj, generic.ErrSkip
	}

	clusterName := infraObj.meta.GetLabels()[capi.ClusterLabelName]
	if clusterName == "" {
		return obj, fmt.Errorf("error retrieving the clustername for machine, label key %s does not appear to exist for dynamic machine %s", capi.ClusterLabelName, key)
	}

	machine, err := rke2.GetMachineByOwner(h.machines, infraObj.meta)
	if err != nil && !errors.Is(err, rke2.ErrNoMachineOwnerRef) {
		return obj, err
	}

	if machine == nil || machine.Status.NodeRef == nil {
		// Machine noderef is nil, we should just allow deletion.
		logrus.Debugf("[MachineProvision] There was no associated K8s node with this etcd dynamicmachine %s. proceeding with deletion", key)
		return h.doRemove(infraObj)
	}

	cluster, err := h.rancherClusterCache.Get(infraObj.meta.GetNamespace(), clusterName)
	if err != nil && !apierrors.IsNotFound(err) {
		return obj, err
	}
	if apierrors.IsNotFound(err) || !cluster.DeletionTimestamp.IsZero() {
		return h.doRemove(infraObj)
	}

	removed := true
	// In the event we are removing an etcd node (as indicated by the etcd-role label on the node), we must safely remove the etcd node from the cluster before allowing machine deprovisioning
	if val := infraObj.meta.GetLabels()["rke.cattle.io/etcd-role"]; val == "true" {
		// we need to block removal until our the v1 node that corresponds has been removed
		restConfig, err := h.kubeconfigManager.GetRESTConfig(cluster, cluster.Status)
		if err != nil {
			return obj, err
		}

		removed, err = etcdmgmt.SafelyRemoved(restConfig, rke2.GetRuntimeCommand(cluster.Spec.KubernetesVersion), machine.Status.NodeRef.Name)
		if err != nil {
			return obj, err
		}
	}

	if _, nodeDrainExcluded := machine.Annotations[capi.ExcludeNodeDrainingAnnotation]; !removed ||
		(!nodeDrainExcluded && !drainingSucceededCondition.IsTrue(machine) && drainingSucceededCondition.GetReason(machine) != capi.DrainingFailedReason) {
		if err = h.dynamic.EnqueueAfter(obj.GetObjectKind().GroupVersionKind(), infraObj.meta.GetNamespace(), infraObj.meta.GetName(), 5*time.Second); err != nil {
			return obj, err
		}
		return obj, generic.ErrSkip
	}

	return h.doRemove(infraObj)
}

func (h *handler) doRemove(infraObj *infraObject) (runtime.Object, error) {
	obj, err := h.run(infraObj, false)
	if err != nil {
		return nil, err
	}

	// ErrSkip will not remove finalizer but treat this as currently reconciled
	return obj, generic.ErrSkip
}

func (h *handler) OnChange(obj runtime.Object) (runtime.Object, error) {
	infraObj, err := newInfraObject(obj)
	if err != nil {
		return nil, err
	}

	// don't process create if deleting
	if !infraObj.meta.GetDeletionTimestamp().IsZero() {
		return obj, nil
	}

	newObj, err := h.run(infraObj, true)
	if newObj == nil {
		newObj = obj
	}

	if err != nil {
		return setCondition(h.dynamic, newObj, createJobConditionType, err)
	}
	return newObj, nil
}

func (h *handler) run(infraObj *infraObject, create bool) (runtime.Object, error) {
	args := infraObj.data.Map("spec")
	driver := getNodeDriverName(infraObj.typeMeta)

	dArgs, err := h.getArgsEnvAndStatus(infraObj, args, driver, create)
	if err != nil {
		return infraObj.obj, err
	}

	if dArgs.BootstrapSecretName == "" && dArgs.BootstrapRequired {
		return infraObj.obj,
			h.dynamic.EnqueueAfter(infraObj.obj.GetObjectKind().GroupVersionKind(), infraObj.meta.GetNamespace(), infraObj.meta.GetName(), 2*time.Second)
	}

	if err := h.apply.WithOwner(infraObj.obj).ApplyObjects(objects(args.String("providerID") != "" && create, dArgs)...); err != nil {
		return nil, err
	}

	if create {
		return h.patchStatus(infraObj.obj, infraObj.data, dArgs.RKEMachineStatus)
	}

	return infraObj.obj, nil
}

func (h *handler) patchStatus(obj runtime.Object, d data.Object, state rkev1.RKEMachineStatus) (runtime.Object, error) {
	statusData, err := convert.EncodeToMap(state)
	if err != nil {
		return nil, err
	}

	if state.JobComplete {
		// Reset failureMessage and failureReason if they are not provided.
		if _, ok := statusData["failureMessage"]; !ok {
			statusData["failureMessage"] = ""
		}
		if _, ok := statusData["failureReason"]; !ok {
			statusData["failureReason"] = ""
		}
	}

	changed := false
	for k, v := range statusData {
		if k != "conditions" {
			if d.String("status", k) != convert.ToString(v) {
				changed = true
			}
		} else if len(state.Conditions) > 0 {
			for _, c := range state.Conditions {
				if thisChanged, err := insertOrUpdateCondition(d, summary.NewCondition(c.Type, string(c.Status), c.Reason, c.Message)); err != nil {
					return nil, err
				} else if thisChanged {
					changed = true
				}
			}
		}
	}

	if !changed {
		return obj, nil
	}

	status := d.Map("status")
	if status == nil {
		status = map[string]interface{}{}
		d.Set("status", status)
	}
	for k, v := range statusData {
		if k != "conditions" {
			status[k] = v
		}
	}

	return h.dynamic.UpdateStatus(&unstructured.Unstructured{
		Object: d,
	})
}

func (h *handler) getJobFromInfraMachine(infraObj *infraObject) (*batchv1.Job, error) {
	gvk := infraObj.obj.GetObjectKind().GroupVersionKind()
	jobs, err := h.jobs.List(infraObj.meta.GetNamespace(), labels.Set{
		InfraMachineGroup:   gvk.Group,
		InfraMachineVersion: gvk.Version,
		InfraMachineKind:    gvk.Kind,
		InfraMachineName:    infraObj.meta.GetName()}.AsSelector(),
	)
	if err != nil {
		return nil, err
	} else if len(jobs) == 0 {
		// This is likely the name of the job, expect if the infra machine object has a very long name.
		return nil, apierrors.NewNotFound(batchv1.Resource("jobs"), GetJobName(infraObj.meta.GetName()))
	}

	// There can be at most one job returned here because there can be at most one infra machine object with the given GVK and name.
	return jobs[0], nil
}

func setCondition(dynamic *dynamic.Controller, obj runtime.Object, conditionType string, err error) (runtime.Object, error) {
	if errors.Is(generic.ErrSkip, err) {
		return obj, nil
	}

	var (
		reason  = ""
		status  = "True"
		message = ""
	)

	if err != nil {
		reason = "Error"
		status = "False"
		message = err.Error()
	}

	desiredCondition := summary.NewCondition(conditionType, status, reason, message)

	d, mapErr := data.Convert(obj.DeepCopyObject())
	if mapErr != nil {
		return obj, mapErr
	}

	if updated, err := insertOrUpdateCondition(d, desiredCondition); !updated || err != nil {
		return obj, err
	}

	obj, updateErr := dynamic.UpdateStatus(&unstructured.Unstructured{
		Object: d,
	})
	if updateErr != nil {
		return obj, updateErr
	}
	return obj, err
}

func insertOrUpdateCondition(d data.Object, desiredCondition summary.Condition) (bool, error) {
	for _, cond := range summary.GetUnstructuredConditions(d) {
		if desiredCondition.Equals(cond) {
			return false, nil
		}
	}

	// The conditions must be converted to a map so that DeepCopyJSONValue will
	// recognize it as a map instead of a data.Object.
	newCond, err := convert.EncodeToMap(desiredCondition.Object)
	if err != nil {
		return false, err
	}

	dConditions := d.Slice("status", "conditions")
	conditions := make([]interface{}, len(dConditions))
	found := false
	for i, cond := range dConditions {
		if cond.String("type") == desiredCondition.Type() {
			conditions[i] = newCond
			found = true
		} else {
			conditions[i], err = convert.EncodeToMap(cond)
			if err != nil {
				return false, err
			}
		}
	}

	if !found {
		conditions = append(conditions, newCond)
	}
	d.SetNested(conditions, "status", "conditions")

	return true, nil
}

func getCondition(d data.Object, conditionType string) *summary.Condition {
	for _, cond := range summary.GetUnstructuredConditions(d) {
		if cond.Type() == conditionType {
			return &cond
		}
	}

	return nil
}

func constructFilesSecret(driver string, config map[string]interface{}) *corev1.Secret {
	secretData := make(map[string][]byte)
	// Check if the required driver has aliased fields
	if fields, ok := node.SchemaToDriverFields[driver]; ok {
		for schemaField, driverField := range fields {
			if fileContents, ok := config[schemaField].(string); ok {
				// Delete our aliased fields
				delete(config, schemaField)
				if fileContents == "" {
					continue
				}

				fileName := driverField
				if ok := nodedriver.SSHKeyFields[schemaField]; ok {
					fileName = "id_rsa"
				}

				// The ending newline gets stripped, add em back
				if !strings.HasSuffix(fileContents, "\n") {
					fileContents = fileContents + "\n"
				}

				// Add the file to the secret
				secretData[fileName] = []byte(fileContents)
				// Add the field and path
				config[driverField] = path.Join(pathToMachineFiles, fileName)
			}
		}
		return &corev1.Secret{Data: secretData}
	}
	return nil
}

func shouldCleanupObjects(job *batchv1.Job, d data.Object) bool {
	if !job.Status.CompletionTime.IsZero() {
		return true
	}

	createJobCondition := getCondition(d, createJobConditionType)
	if createJobCondition == nil {
		return false
	}

	forceRemoveAnnValue, _ := d.Map("metadata", "annotations")[forceRemoveMachineAnn].(string)

	if createJobCondition.Reason() == string(capierrors.CreateMachineError) || strings.ToLower(forceRemoveAnnValue) == "true" {
		return strings.ToLower(job.Spec.Template.Labels[InfraJobRemove]) == "true" && condition.Cond("Failed").IsTrue(job)
	}

	return false
}

func FailedMachineDeleteMessage(kind, failureReason, failureMessage string) string {
	return fmt.Sprintf("failed deleting server (%s) in infrastructure provider: %s: %s", kind, failureReason, failureMessage)
}

func FailedMachineCreateMessage(kind, failureReason, failureMessage string) string {
	return fmt.Sprintf("failed creating server (%s) in infrastructure provider: %s: %s", kind, failureReason, failureMessage)
}

func CreatingMachineMessage(kind string) string {
	return fmt.Sprintf("creating server (%s) in infrastructure provider", kind)
}

func DeletingMachineMessage(kind string) string {
	return fmt.Sprintf("deleting server (%s) in infrastructure provider", kind)
}
