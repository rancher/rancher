package machineprovision

import (
	"context"
	errors2 "errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/provisioningv2/kubeconfig"
	v2provruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"

	"github.com/rancher/lasso/pkg/dynamic"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/controllers/management/drivers/nodedriver"
	"github.com/rancher/rancher/pkg/controllers/management/node"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	ranchercontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
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
	ctx                 context.Context
	apply               apply.Apply
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

	newStatus, err := h.getMachineStatus(job, job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit == 0)
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

func (h *handler) getMachine(obj runtime.Object) (*capi.Machine, error) {
	var (
		machine *capi.Machine
		err     error
	)

	meta, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	for _, owner := range meta.GetOwnerReferences() {
		if owner.Kind == "Machine" {
			machine, err = h.machines.Get(meta.GetNamespace(), owner.Name)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return machine, nil

}

func (h *handler) getMachineStatus(job *batchv1.Job, create bool) (rkev1.RKEMachineStatus, error) {
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
			return getMachineStatusFromPod(lastPod, create), nil
		}
	}

	return rkev1.RKEMachineStatus{}, nil
}

func getMachineStatusFromPod(pod *corev1.Pod, create bool) rkev1.RKEMachineStatus {
	if pod.Status.Phase == corev1.PodSucceeded {
		return rkev1.RKEMachineStatus{
			JobComplete: true,
		}
	}

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
			var reason string
			if create {
				reason = string(errors.CreateMachineError)
			} else {
				reason = string(errors.DeleteMachineError)
			}
			return rkev1.RKEMachineStatus{
				FailureReason:  reason,
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

func (h *handler) OnRemove(key string, obj runtime.Object) (runtime.Object, error) {
	if removed, err := h.namespaceIsRemoved(obj); err != nil || removed {
		return obj, err
	}

	d, err := data.Convert(obj.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	if val, ok := d.Map("metadata", "labels")["rke.cattle.io/etcd-role"]; ok {
		if val.(string) == "true" {
			// we need to block removal until our the v1 node that corresponds has been removed
			clusterName := d.Map("metadata", "labels")[CapiMachineLabel].(string)
			if clusterName == "" {
				logrus.Errorf("MachineProvision There was an error retrieving the clustername for this etcd node")
				return obj, fmt.Errorf("nope")
			}
			cluster, err := h.rancherClusterCache.Get(d.String("metadata", "namespace"), clusterName)
			if apierror.IsNotFound(err) {
				// we can go ahead and remove
				logrus.Infof("MachineProvision Proceeding with removal of node as cluster was not found.")
				return h.doRemove(obj)
			} else if err != nil {
				return obj, err
			}
			if !cluster.DeletionTimestamp.IsZero() {
				// If the cluster deletion timestamp has been set, we can blindly proceed with delete.
				logrus.Infof("MachineProvision cluster deletion timestamp was not zero. proceeding with delete")
				return h.doRemove(obj)
			}
			restConfig, err := h.kubeconfigManager.GetRESTConfig(cluster, cluster.Status)
			if err != nil {
				return obj, err
			}

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				return obj, err
			}

			logrus.Infof("MachineProvision built k8s clientset")

			removeAnnotation := "etcd." + v2provruntime.GetRuntimeCommand(cluster.Spec.KubernetesVersion) + ".cattle.io/remove"
			removedNodeNameAnnotation := "etcd." + v2provruntime.GetRuntimeCommand(cluster.Spec.KubernetesVersion) + ".cattle.io/removed-node-name"

			machine, err := h.getMachine(obj)

			if err != nil {
				return obj, err
			}

			if machine.Status.NodeRef == nil {
				// Machine noderef is nil, we should just allow deletion.
				logrus.Infof("MachineProvision there was no associated node with this etcd node. proceeding with deletion")
				return h.doRemove(obj)
			}

			logrus.Infof("MachineProvision getting node %s for the dynamic machine %s", machine.Status.NodeRef.Name, key)

			node, err := clientset.CoreV1().Nodes().Get(context.TODO(), machine.Status.NodeRef.Name, metav1.GetOptions{})
			if err != nil {
				if apierror.IsNotFound(err) {
					logrus.Infof("MachineProvision Node not found. proceeding with delete")
					return h.doRemove(obj)
				}
				return obj, err
			}

			if val, ok := node.Annotations[removeAnnotation]; ok {
				// check val to see if it's true, if not, continue
				if val == "true" {
					// check the status of the removal
					logrus.Infof("MachineProvision etcd removal is already in progress as per the annotation")
					if removedNodeName, ok := node.Annotations[removedNodeNameAnnotation]; ok {
						// There is the possibility the annotation is defined, but empty.
						if removedNodeName != "" {
							return h.doRemove(obj)
						}
					}
					return obj, fmt.Errorf("remove annotation already set, waiting for node removal to be successful")
				}
			}
			// The remove annotation has not been set to true, so we'll go ahead and set it on the node.
			err = retry.RetryOnConflict(retry.DefaultRetry,
				func() error {
					node.Annotations[removeAnnotation] = "true"
					node, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
					return err
				})
			if err != nil {
				// there was an error updating the node
				return obj, err
			}

			return obj, fmt.Errorf("waiting for etcd member removal")
		}
	}
	return h.doRemove(obj)
}

func (h *handler) doRemove(obj runtime.Object) (runtime.Object, error) {
	logrus.Infof("MachineProvision doing removal now")
	d, err := data.Convert(obj.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	infraName := d.String("metadata", "name")
	if !d.Bool("status", "jobComplete") && d.String("status", "failureReason") == "" {
		return obj, fmt.Errorf("cannot delete machine %s because create job has not finished", infraName)
	}

	obj, err = h.run(obj, false)
	if err != nil {
		return nil, err
	}

	job, err := h.jobs.Get(d.String("metadata", "namespace"), getJobName(infraName))
	if err != nil {
		return nil, err
	}

	if job.Status.CompletionTime != nil {
		// Calling WithOwner(obj).ApplyObjects with no objects here will look for all objects with types passed to
		// WithCacheTypes above that have an owner label (not owner reference) to the given obj. It will compare the existing
		// objects it finds to the ones that are passed to ApplyObjects (which there are none in this case). The apply
		// controller will delete all existing objects it finds that are not passed to ApplyObjects. Since no objects are
		// passed here, it will delete all objects it finds.
		if err := h.apply.WithOwner(obj).ApplyObjects(); err != nil {
			return nil, err
		}
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

	d, err := data.Convert(obj.DeepCopyObject())
	if err != nil {
		return nil, err
	}

	args := d.Map("spec")
	driver := getNodeDriverName(typeMeta)

	filesSecret, err := constructFilesSecret(driver, args)
	if err != nil {
		return obj, err
	}

	dArgs, err := h.getArgsEnvAndStatus(meta, d, args, driver, create)
	if err != nil {
		return obj, err
	}

	if dArgs.BootstrapSecretName == "" && !dArgs.BootstrapOptional {
		return obj,
			h.dynamic.EnqueueAfter(obj.GetObjectKind().GroupVersionKind(), meta.GetNamespace(), meta.GetName(), 2*time.Second)
	}

	objs, err := h.objects(d.Bool("status", "ready") && create, typeMeta, meta, dArgs, filesSecret)
	if err != nil {
		return nil, err
	}

	applier := h.apply.WithOwner(obj)
	if !create {
		// If the infrastructure is being deleted, ignore previously applied objects.
		// If creation failed, this will allow deletion to process.
		applier = applier.WithIgnorePreviousApplied()
	}
	if err := applier.ApplyObjects(objs...); err != nil {
		return nil, err
	}

	if create {
		return h.patchStatus(obj, d, dArgs.RKEMachineStatus)
	}

	return obj, nil
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

func constructFilesSecret(driver string, config map[string]interface{}) (*corev1.Secret, error) {
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
		return &corev1.Secret{Data: secretData}, nil
	}
	return nil, nil
}
