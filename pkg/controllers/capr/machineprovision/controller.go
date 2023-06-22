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
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/controllers/management/drivers/nodedriver"
	"github.com/rancher/rancher/pkg/controllers/management/node"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	batchcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/genericcondition"
	"github.com/rancher/wrangler/pkg/name"
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
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
)

const (
	createJobConditionType = "CreateJob"
	deleteJobConditionType = "DeleteJob"

	forceRemoveMachineAnn = "provisioning.cattle.io/force-machine-remove"
)

type infraObject struct {
	data     data.Object
	obj      runtime.Object
	meta     metav1.Object
	typeMeta metav1.Type
}

func newInfraObject(obj runtime.Object) (*infraObject, error) {
	copiedObj := obj.DeepCopyObject()
	objMeta, err := meta.Accessor(copiedObj)
	if err != nil {
		return nil, err
	}

	data, err := data.Convert(copiedObj)
	if err != nil {
		return nil, err
	}

	typeMeta, err := meta.TypeAccessor(copiedObj)
	if err != nil {
		return nil, err
	}

	return &infraObject{
		data:     data,
		obj:      copiedObj,
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
	capiClusterCache    capicontrollers.ClusterCache
	machineCache        capicontrollers.MachineCache
	machineClient       capicontrollers.MachineClient
	machineSetCache     capicontrollers.MachineSetCache
	namespaces          corecontrollers.NamespaceCache
	nodeDriverCache     mgmtcontrollers.NodeDriverCache
	dynamic             *dynamic.Controller
	rancherClusterCache ranchercontrollers.ClusterCache
	kubeconfigManager   *kubeconfig.Manager
}

func Register(ctx context.Context, clients *wrangler.Context, kubeconfigManager *kubeconfig.Manager) {
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
		machineCache:        clients.CAPI.Machine().Cache(),
		machineClient:       clients.CAPI.Machine(),
		machineSetCache:     clients.CAPI.MachineSet().Cache(),
		capiClusterCache:    clients.CAPI.Cluster().Cache(),
		nodeDriverCache:     clients.Mgmt.NodeDriver().Cache(),
		namespaces:          clients.Core.Namespace().Cache(),
		dynamic:             clients.Dynamic,
		rancherClusterCache: clients.Provisioning.Cluster().Cache(),
		kubeconfigManager:   kubeconfigManager,
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

func (h *handler) OnJobChange(_ string, job *batchv1.Job) (*batchv1.Job, error) {
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

	infra, err := newInfraObject(infraMachine)
	if err != nil {
		return job, err
	}

	if infra.data.String("status", "jobName") == "" {
		infra.data.SetNested(job.Name, "status", "jobName")
		_, err = h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: infra.data,
		})
		return job, err
	}

	// Re-evaluate the infra-machine after this
	if err = h.dynamic.Enqueue(infraMachine.GetObjectKind().GroupVersionKind(),
		infra.meta.GetNamespace(), infra.meta.GetName()); err != nil {
		return job, err
	}

	return job, nil
}

func (h *handler) getMachineStatus(job *batchv1.Job) (rkev1.RKEMachineStatus, error) {
	condType := createJobConditionType
	if job.Spec.Template.Labels[InfraJobRemove] == "true" {
		condType = deleteJobConditionType
	}

	if condition.Cond("Complete").IsTrue(job) {
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
		}, nil
	} else if condition.Cond("Failed").IsTrue(job) {
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
			return getMachineStatusFromPod(lastPod, condType), nil
		}
	}

	return rkev1.RKEMachineStatus{Conditions: []genericcondition.GenericCondition{
		{
			Type:    "Ready",
			Status:  corev1.ConditionFalse,
			Message: ExecutingMachineMessage(job.Spec.Template.Labels, job.Namespace),
		},
	}}, nil
}

func getMachineStatusFromPod(pod *corev1.Pod, condType string) rkev1.RKEMachineStatus {
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
		}
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
			failureMessage := strings.TrimSpace(containerStatus.State.Terminated.Message)
			message := FailedMachineMessage(pod.Labels, pod.Namespace, reason, failureMessage)
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

	infra, err := newInfraObject(obj)
	if err != nil {
		return obj, err
	}

	// When infra machines are initially created, the machine set controller sets itself as the owner reference
	// Later, the CAPI machine will adopt the node, setting itself as the owner reference
	// If the machine set is still present as the owner reference, just delete the machine since no provisioning will have taken place yet
	if machineSet, _ := capr.GetOwnerCAPIMachineSet(infra.obj, h.machineSetCache); machineSet != nil {
		return obj, nil
	}

	// Initial provisioning not finished
	if cond := getCondition(infra.data, createJobConditionType); cond != nil && cond.Status() == "Unknown" {
		job, err := h.getJobFromInfraMachine(infra)
		if apierrors.IsNotFound(err) {
			// If the job is not found, go ahead and proceed with machine deletion
			return obj, h.apply.WithOwner(obj).ApplyObjects()
		} else if err != nil {
			return obj, err
		}
		logrus.Debugf("[machineprovision] create job for %s not finished, job was found and the error was not nil and was not an isnotfound", key)
		// OnChange handler will not run when the infra machine is being deleted, we have to reconcile here in order to
		// finish the create job, since it has to have completed successfully or never ran for the delete job to run
		state, _, err := h.run(infra, true)
		if err != nil {
			return obj, err
		}
		if err = reconcileStatus(infra.data, state); err != nil {
			return obj, err
		}
		if job != nil {
			newStatus, err := h.getMachineStatus(job)
			if err != nil {
				return obj, err
			}
			newStatus.JobName = job.Name

			err = reconcileStatus(infra.data, newStatus)
			if err != nil {
				return obj, err
			}
		}
		if obj, err = h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: infra.data,
		}); err != nil {
			return obj, err
		}
		return obj, fmt.Errorf("cannot delete machine %s because create job has not finished", infra.meta.GetName())
	} else if cond == nil {
		// If the createJobCondition is not set on this infra object, that means we never actually tried to run a create job
		// so there should be no infrastructure, proceed with delete
		return obj, nil
	}

	// infrastructure deletion finished
	if cond := getCondition(infra.data, deleteJobConditionType); cond != nil && cond.Status() != "Unknown" {
		job, err := h.getJobFromInfraMachine(infra)
		if apierrors.IsNotFound(err) {
			// If the deletion job condition has been set on the infrastructure object and the deletion job has been removed,
			// then we don't want to create another deletion job.
			logrus.Infof("[machineprovision] Machine %s %s has already been deleted", infra.obj.GetObjectKind().GroupVersionKind(), infra.meta.GetName())
			return obj, h.apply.WithOwner(obj).ApplyObjects()
		} else if err != nil {
			return obj, err
		}

		if shouldCleanupObjects(job, infra.data) {
			// Calling WithOwner(obj).ApplyObjects with no objects here will look for all objects with types passed to
			// WithCacheTypes above that have an owner label (not owner reference) to the given obj. It will compare the existing
			// objects it finds to the ones that are passed to ApplyObjects (which there are none in this case). The apply
			// controller will delete all existing objects it finds that are not passed to ApplyObjects. Since no objects are
			// passed here, it will delete all objects it finds.
			return obj, h.apply.WithOwner(obj).ApplyObjects()
		}
		if cond.Status() == "True" {
			return obj, generic.ErrSkip
		}
	}

	clusterName := infra.meta.GetLabels()[capi.ClusterLabelName]
	if clusterName == "" {
		return obj, fmt.Errorf("error retrieving the clustername for machine, label key %s does not appear to exist for dynamic machine %s", capi.ClusterLabelName, key)
	}

	machine, err := capr.GetOwnerCAPIMachine(obj, h.machineCache)
	if err != nil && !errors.Is(err, capr.ErrNoMatchingControllerOwnerRef) && !apierrors.IsNotFound(err) {
		logrus.Errorf("[machineprovision] %s/%s: error getting machine by owner reference: %v", infra.meta.GetNamespace(), infra.meta.GetName(), err)
		return obj, err
	}

	// If the controller owner reference is not properly configured, or the CAPI machine does not exist, there is no way
	// to recover from this situation, so we should proceed with deletion
	if machine == nil || machine.Status.NodeRef == nil {
		// Machine noderef is nil, we should just allow deletion.
		logrus.Debugf("[machineprovision] There was no associated K8s node with this machine %s. Proceeding with deletion", key)
		return h.doRemove(infra)
	}

	cluster, err := h.rancherClusterCache.Get(infra.meta.GetNamespace(), clusterName)
	if err != nil && !apierrors.IsNotFound(err) {
		return obj, err
	}
	if apierrors.IsNotFound(err) || !cluster.DeletionTimestamp.IsZero() {
		return h.doRemove(infra)
	}

	return h.doRemove(infra)
}

func (h *handler) doRemove(infra *infraObject) (runtime.Object, error) {
	state, _, err := h.run(infra, false)
	if err != nil {
		return infra.obj, err
	}

	if err = reconcileStatus(infra.data, state); err != nil {
		return infra.obj, err
	}

	if cond := getCondition(infra.data, deleteJobConditionType); cond == nil {
		if err = reconcileStatus(infra.data, rkev1.RKEMachineStatus{
			Conditions: []genericcondition.GenericCondition{
				{
					Type:    deleteJobConditionType,
					Status:  corev1.ConditionUnknown,
					Message: "creating machine deletion job",
				},
			}}); err != nil {
			return infra.obj, err
		}
		if infra.obj, err = h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: infra.data,
		}); err != nil {
			return infra.obj, err
		}
		return infra.obj, generic.ErrSkip
	}

	jobName := infra.data.String("status", "jobName")
	if jobName == "" {
		return infra.obj, generic.ErrSkip
	}

	job, err := h.jobs.Get(infra.meta.GetNamespace(), jobName)
	if apierrors.IsNotFound(err) {
		if infra.obj, err = h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: infra.data,
		}); err != nil {
			return infra.obj, err
		}
		return infra.obj, generic.ErrSkip
	} else if err != nil {
		return infra.obj, err
	}

	newStatus, err := h.getMachineStatus(job)
	if err != nil {
		return infra.obj, err
	}
	newStatus.JobName = job.Name

	err = reconcileStatus(infra.data, newStatus)
	if err != nil {
		return infra.obj, err
	}

	// We need to reset failureReason & failureMessage, otherwise the deletion status will not be shown
	if infra.data.String("status", "failureReason") == string(capierrors.CreateMachineError) {
		infra.data.SetNested("", "status", "failureReason")
		infra.data.SetNested("", "status", "failureMessage")
	}

	if infra.obj, err = h.dynamic.UpdateStatus(&unstructured.Unstructured{
		Object: infra.data,
	}); err != nil {
		return infra.obj, err
	}

	return infra.obj, generic.ErrSkip
}

func (h *handler) EnqueueAfter(infra *infraObject, duration time.Duration) {
	err := h.dynamic.EnqueueAfter(infra.obj.GetObjectKind().GroupVersionKind(), infra.meta.GetNamespace(), infra.meta.GetName(), duration)
	if err != nil {
		logrus.Errorf("[machineprovision] error enqueuing %s %s/%s: %v", infra.obj.GetObjectKind().GroupVersionKind(), infra.meta.GetNamespace(), infra.meta.GetName(), err)
	}
}

// OnChange is called whenever the infrastructure machine is updated, including when the object is being deleted.
func (h *handler) OnChange(obj runtime.Object) (runtime.Object, error) {
	infra, err := newInfraObject(obj)
	if err != nil {
		return obj, err
	}

	if !infra.meta.GetDeletionTimestamp().IsZero() {
		return obj, nil
	}

	machine, err := capr.GetOwnerCAPIMachine(obj, h.machineCache)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[machineprovision] %s/%s: waiting: machine to be set as owner reference", infra.meta.GetNamespace(), infra.meta.GetName())
		h.EnqueueAfter(infra, 10*time.Second)
		return obj, generic.ErrSkip
	}

	if err != nil {
		logrus.Errorf("[machineprovision] %s/%s: error getting machine by owner reference: %v", infra.meta.GetNamespace(), infra.meta.GetName(), err)
		return obj, err
	}

	// If the CAPI machine is deleted forcefully, the owner reference will not be usable, so the label allows us to create
	// the delete job with the same name
	if infra.meta.GetLabels()[CapiMachineName] == "" {
		infra.data.SetNested(machine.Name, "metadata", "labels", CapiMachineName)
		// Return prematurely, we want the caches to be as up-to-date as possible, so that we don't lose changes when
		// reconciling in the event of other errors
		return h.dynamic.Update(&unstructured.Unstructured{
			Object: infra.data,
		})
	}

	capiCluster, err := capr.GetCAPIClusterFromLabel(machine, h.capiClusterCache)
	if apierrors.IsNotFound(err) {
		logrus.Debugf("[machineprovision] %s/%s: waiting: CAPI cluster does not exist", infra.meta.GetNamespace(), infra.meta.GetName())
		h.EnqueueAfter(infra, 10*time.Second)
		return obj, generic.ErrSkip
	}
	if err != nil {
		logrus.Errorf("[machineprovision] %s/%s: error getting CAPI cluster %v", infra.meta.GetNamespace(), infra.meta.GetName(), err)
		return obj, err
	}

	if capiannotations.IsPaused(capiCluster, infra.meta) {
		logrus.Debugf("[machineprovision] %s/%s: waiting: CAPI cluster or RKEMachine is paused", infra.meta.GetNamespace(), infra.meta.GetName())
		h.EnqueueAfter(infra, 10*time.Second)
		return obj, generic.ErrSkip
	}

	if !capiCluster.Status.InfrastructureReady {
		logrus.Debugf("[machineprovision] %s/%s: waiting: CAPI cluster infrastructure is not ready", infra.meta.GetNamespace(), infra.meta.GetName())
		h.EnqueueAfter(infra, 10*time.Second)
		return obj, generic.ErrSkip
	}

	if machine.Spec.Bootstrap.DataSecretName == nil {
		logrus.Debugf("[machineprovision] %s/%s: waiting: dataSecretName is not populated on machine spec", infra.meta.GetNamespace(), infra.meta.GetName())
		h.EnqueueAfter(infra, 10*time.Second)
		return obj, generic.ErrSkip
	}

	state, failure, err := h.run(infra, true)
	if err != nil {
		return obj, err
	}

	if failure {
		logrus.Infof("[machineprovision] %s/%s: Failed to create infrastructure for machine %s, deleting and recreating...", infra.meta.GetNamespace(), infra.meta.GetName(), machine.Name)
		if err = h.machineClient.Delete(machine.Namespace, machine.Name, &metav1.DeleteOptions{}); err != nil {
			return obj, err
		}
	}

	if err = reconcileStatus(infra.data, state); err != nil {
		return obj, err
	}

	if cond := getCondition(infra.data, createJobConditionType); cond == nil {
		if err = reconcileStatus(infra.data, rkev1.RKEMachineStatus{
			Conditions: []genericcondition.GenericCondition{
				{
					Type:    createJobConditionType,
					Status:  corev1.ConditionUnknown,
					Message: "creating machine provision job",
				},
			}}); err != nil {
			return obj, err
		}
		return h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: infra.data,
		})
	}

	jobName := infra.data.String("status", "jobName")
	if jobName == "" {
		return obj, nil
	}

	job, err := h.jobs.Get(infra.meta.GetNamespace(), jobName)
	if apierrors.IsNotFound(err) {
		return h.dynamic.UpdateStatus(&unstructured.Unstructured{
			Object: infra.data,
		})
	} else if err != nil {
		return obj, err
	}

	newStatus, err := h.getMachineStatus(job)
	if err != nil {
		return obj, err
	}
	newStatus.JobName = job.Name

	err = reconcileStatus(infra.data, newStatus)
	if err != nil {
		return obj, err
	}

	return h.dynamic.UpdateStatus(&unstructured.Unstructured{
		Object: infra.data,
	})
}

func (h *handler) run(infra *infraObject, create bool) (rkev1.RKEMachineStatus, bool, error) {
	logrus.Infof("[machineprovision] %s/%s: reconciling machine job", infra.meta.GetNamespace(), infra.meta.GetName())

	args := infra.data.Map("spec")
	driver := getNodeDriverName(infra.typeMeta)

	dArgs, err := h.getArgsEnvAndStatus(infra, args, driver, create)
	if err != nil {
		return rkev1.RKEMachineStatus{}, false, err
	}

	if dArgs.BootstrapSecretName == "" && dArgs.BootstrapRequired {
		return rkev1.RKEMachineStatus{}, false,
			h.dynamic.EnqueueAfter(infra.obj.GetObjectKind().GroupVersionKind(), infra.meta.GetNamespace(), infra.meta.GetName(), 2*time.Second)
	}

	failureReasonType := capierrors.CreateMachineError
	if !create {
		failureReasonType = capierrors.DeleteMachineError
	}

	// Check to see if we have a failure reason.
	failure := infra.data.String("status", "failureReason") == string(failureReasonType)
	ready := false

	cond := getCondition(infra.data, "Ready")
	if cond != nil {
		// We are only "ready" if both the condition "Ready" is "True" and the provider ID has been set on the machine.
		ready = cond.Status() == "True" && args.String("providerID") != ""
	}

	if err := h.apply.WithOwner(infra.obj).ApplyObjects(objects((ready || failure) && create, dArgs)...); err != nil {
		return rkev1.RKEMachineStatus{}, failure, err
	}

	return dArgs.RKEMachineStatus, failure, err
}

// reconcileStatus will update the infra machine's status by updating each field based on the value of status.
func reconcileStatus(d data.Object, state rkev1.RKEMachineStatus) error {
	statusData, err := convert.EncodeToMap(state)
	if err != nil {
		return err
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
					return err
				} else if thisChanged {
					changed = true
				}
			}
		}
	}

	if !changed {
		return nil
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
	return nil
}

func (h *handler) getJobFromInfraMachine(infra *infraObject) (*batchv1.Job, error) {
	gvk := infra.obj.GetObjectKind().GroupVersionKind()
	jobs, err := h.jobs.List(infra.meta.GetNamespace(), labels.Set{
		InfraMachineGroup:   gvk.Group,
		InfraMachineVersion: gvk.Version,
		InfraMachineKind:    gvk.Kind,
		InfraMachineName:    infra.meta.GetName()}.AsSelector(),
	)
	if err != nil {
		return nil, err
	} else if len(jobs) == 0 {
		// This is likely the name of the job, expect if the infra machine object has a very long name.
		return nil, apierrors.NewNotFound(batchv1.Resource("jobs"), GetJobName(infra.meta.GetName()))
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

func (h *handler) constructCertsSecret(machineName, machineNamespace string) (*corev1.Secret, error) {
	certSecretData := make(map[string][]byte)

	cert := settings.CACerts.Get()
	if cert != "" {
		certSecretData["tls.crt"] = []byte(cert)
	}

	cert = settings.InternalCACerts.Get()
	if cert != "" {
		certSecretData["internal-tls.crt"] = []byte(cert)
	}

	if secret, err := h.secrets.Get(namespace.System, "tls-additional"); err == nil {
		for key, val := range secret.Data {
			certSecretData[key] = val
		}
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	if len(certSecretData) > 0 {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.SafeConcatName("machine", "certs", hashName(machineName)),
				Namespace: machineNamespace,
			},
			Data: certSecretData,
		}, nil
	}

	return nil, nil
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

func FailedMachineMessage(podLabels map[string]string, namespace, failureReason, failureMessage string) string {
	verb := "creating"
	if podLabels[InfraJobRemove] == "true" {
		verb = "deleting"
	}
	return fmt.Sprintf("failed %s server [%s/%s] of kind (%s) for machine %s in infrastructure provider: %s: %s",
		verb,
		namespace,
		podLabels[InfraMachineName],
		podLabels[InfraMachineKind],
		podLabels[CapiMachineName],
		failureReason,
		failureMessage,
	)
}

func ExecutingMachineMessage(podLabels map[string]string, namespace string) string {
	verb := "creating"
	if podLabels[InfraJobRemove] == "true" {
		verb = "deleting"
	}
	return fmt.Sprintf("%s server [%s/%s] of kind (%s) for machine %s in infrastructure provider",
		verb,
		namespace,
		podLabels[InfraMachineName],
		podLabels[InfraMachineKind],
		podLabels[CapiMachineName],
	)
}
