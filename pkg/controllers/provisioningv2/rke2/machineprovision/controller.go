package machineprovision

import (
	"context"
	errors2 "errors"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	"github.com/rancher/wrangler/pkg/data/convert"
	batchcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/batch/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/summary"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cluster-api/errors"
)

type handler struct {
	ctx             context.Context
	apply           apply.Apply
	jobs            batchcontrollers.JobCache
	pods            corecontrollers.PodCache
	secrets         corecontrollers.SecretCache
	machines        capicontrollers.MachineCache
	namespaces      corecontrollers.NamespaceCache
	nodeDriverCache mgmtcontrollers.NodeDriverCache
	dynamic         *dynamic.Controller
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := &handler{
		ctx: ctx,
		apply: clients.Apply.
			WithSetOwnerReference(true, true).
			WithCacheTypes(clients.Core.Secret(),
				clients.Core.ServiceAccount(),
				clients.RBAC.RoleBinding(),
				clients.RBAC.Role(),
				clients.Batch.Job()),
		pods:            clients.Core.Pod().Cache(),
		jobs:            clients.Batch.Job().Cache(),
		secrets:         clients.Core.Secret().Cache(),
		machines:        clients.CAPI.Machine().Cache(),
		nodeDriverCache: clients.Mgmt.NodeDriver().Cache(),
		namespaces:      clients.Core.Namespace().Cache(),
		dynamic:         clients.Dynamic,
	}

	removeHandler := generic.NewRemoveHandler("machine-provision-remove", clients.Dynamic.Update, h.OnRemove)

	clients.Dynamic.OnChange(ctx, "machine-provision-remove", validGVK, dynamic.FromKeyHandler(removeHandler))
	clients.Dynamic.OnChange(ctx, "machine-provision", validGVK, h.OnChange)
	clients.Batch.Job().OnChange(ctx, "machine-provision-pod", h.OnJobChange)
}

func validGVK(gvk schema.GroupVersionKind) bool {
	return gvk.Group == "rke-node.cattle.io" &&
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
	if apierror.IsNotFound(err) {
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

	newStatus, err := h.getMachineStatus(job)
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

func (h *handler) getMachineStatus(job *batchv1.Job) (rkev1.RKEMachineStatus, error) {
	if job.Status.CompletionTime != nil {
		return rkev1.RKEMachineStatus{
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
			return getMachineStatusFromPod(lastPod), nil
		}
	}

	return rkev1.RKEMachineStatus{}, nil
}

func getMachineStatusFromPod(pod *corev1.Pod) rkev1.RKEMachineStatus {
	if pod.Status.Phase == corev1.PodSucceeded {
		return rkev1.RKEMachineStatus{
			JobComplete: true,
		}
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
			return rkev1.RKEMachineStatus{
				FailureReason:  string(errors.CreateMachineError),
				FailureMessage: strings.TrimSpace(pod.Status.ContainerStatuses[0].State.Terminated.Message),
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

func (h *handler) OnRemove(_ string, obj runtime.Object) (runtime.Object, error) {
	if removed, err := h.namespaceIsRemoved(obj); err != nil || removed {
		return obj, err
	}

	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return obj, err
	}

	ref := metav1.GetControllerOf(metaObj)
	if ref != nil && ref.Kind == "Machine" {
		_, err = h.machines.Get(metaObj.GetNamespace(), ref.Name)
		if apierror.IsNotFound(err) {
			// Controlling machine has been deleted (normally a finalizer blocks this)
			return obj, nil
		}
	}

	obj, err = h.run(obj, false)
	if err != nil {
		return nil, err
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	job, err := h.jobs.Get(meta.GetNamespace(), getJobName(meta.GetName()))
	if err != nil {
		return nil, err
	}

	if condition.Cond("Failed").IsTrue(job) || job.Status.CompletionTime != nil {
		return obj, nil
	}

	// ErrSkip will not remove finalizer but treat this as currently reconciled
	return nil, generic.ErrSkip
}

func (h *handler) OnChange(obj runtime.Object) (runtime.Object, error) {
	newObj, err := h.run(obj, true)
	if newObj == nil {
		newObj = obj
	}
	return setCondition(h.dynamic, newObj, "CreateJob", err)
}

func (h *handler) run(obj runtime.Object, create bool) (runtime.Object, error) {
	typeMeta, err := meta.TypeAccessor(obj)
	if err != nil {
		return nil, err
	}

	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	// don't process create if deleting
	if create && meta.GetDeletionTimestamp() != nil {
		return obj, nil
	}

	d, err := data.Convert(obj)
	if err != nil {
		return nil, err
	}

	args, err := h.getArgsEnvAndStatus(typeMeta, meta, d, create)
	if err != nil {
		return obj, err
	}

	if args.BootstrapSecretName == "" && !args.BootstrapOptional {
		return obj,
			h.dynamic.EnqueueAfter(obj.GetObjectKind().GroupVersionKind(), meta.GetNamespace(), meta.GetName(), 2*time.Second)
	}

	objs, err := h.objects(d.Bool("status", "ready") && create, typeMeta, meta, args)
	if err != nil {
		return nil, err
	}

	if err := h.apply.WithOwner(obj).ApplyObjects(objs...); err != nil {
		return nil, err
	}

	if create {
		return h.patchStatus(obj, d, args.RKEMachineStatus)
	}

	return obj, h.apply.WithOwner(obj).ApplyObjects(objs...)
}

func (h *handler) patchStatus(obj runtime.Object, d data.Object, state rkev1.RKEMachineStatus) (runtime.Object, error) {
	statusData, err := convert.EncodeToMap(state)
	if err != nil {
		return nil, err
	}

	changed := false
	for k, v := range statusData {
		if d.String("status", k) != convert.ToString(v) {
			changed = true
			break
		}
	}

	if !changed {
		return obj, nil
	}

	d, err = data.Convert(obj.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	status := d.Map("status")
	if status == nil {
		status = map[string]interface{}{}
		d.Set("status", status)
	}
	for k, v := range statusData {
		status[k] = v
	}

	return h.dynamic.UpdateStatus(&unstructured.Unstructured{
		Object: d,
	})
}

func setCondition(dynamic *dynamic.Controller, obj runtime.Object, conditionType string, err error) (runtime.Object, error) {
	var (
		reason  = ""
		status  = "True"
		message = ""
	)

	if errors2.Is(generic.ErrSkip, err) {
		err = nil
	}

	if err != nil {
		reason = "Error"
		status = "False"
		message = err.Error()
	}

	desiredCondition := summary.NewCondition(conditionType, status, reason, message)

	d, mapErr := data.Convert(obj)
	if mapErr != nil {
		return obj, mapErr
	}

	for _, condition := range summary.GetUnstructuredConditions(d) {
		if condition.Type() == conditionType {
			if desiredCondition.Equals(condition) {
				return obj, err
			}
			break
		}
	}

	d, mapErr = data.Convert(obj.DeepCopyObject())
	if mapErr != nil {
		return obj, mapErr
	}

	conditions := d.Slice("status", "conditions")
	found := false
	for i, condition := range conditions {
		if condition.String("type") == conditionType {
			conditions[i] = desiredCondition.Object
			d.SetNested(conditions, "status", "conditions")
			found = true
		}
	}

	if !found {
		d.SetNested(append(conditions, desiredCondition.Object), "status", "conditions")
	}
	obj, updateErr := dynamic.UpdateStatus(&unstructured.Unstructured{
		Object: d,
	})
	if err != nil {
		return obj, err
	}
	return obj, updateErr
}
