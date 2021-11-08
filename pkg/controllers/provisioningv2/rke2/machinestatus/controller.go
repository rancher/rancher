package machinestatus

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rancher/lasso/pkg/dynamic"
	"github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	capicontrollers "github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1alpha4"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	rkecontroller "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/provisioningv2/rke2/planner"
	rancherruntime "github.com/rancher/rancher/pkg/provisioningv2/rke2/runtime"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/relatedresource"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1alpha4"
	capierror "sigs.k8s.io/cluster-api/errors"
)

const (
	Provisioned         = condition.Cond("Provisioned")
	InfrastructureReady = condition.Cond(capi.InfrastructureReadyCondition)
	BootstrapReady      = condition.Cond(capi.BootstrapReadyCondition)
)

type machineStatus struct {
	cond                        condition.Cond
	status                      corev1.ConditionStatus
	reason, message, providerID string
}

func (m *machineStatus) toCapiCondition() capi.Condition {
	capiCond := capi.Condition{
		Type:               capi.ConditionType(m.cond),
		Status:             m.status,
		LastTransitionTime: metav1.Now(),
		Reason:             m.reason,
		Message:            m.message,
	}
	if m.status == corev1.ConditionFalse {
		capiCond.Severity = capi.ConditionSeverityError
	} else {
		capiCond.Severity = capi.ConditionSeverityInfo
	}

	return capiCond
}

func (m *machineStatus) machineStatusNeedsUpdate(machine *capi.Machine) bool {
	return m.cond.GetStatus(machine) != string(m.status) ||
		m.cond.GetReason(machine) != m.reason ||
		m.cond.GetMessage(machine) != m.message
}

type handler struct {
	secrets              corecontrollers.SecretCache
	machines             capicontrollers.MachineController
	bootstrapCache       rkecontroller.RKEBootstrapCache
	bootstrapController  rkecontroller.RKEBootstrapController
	capiClusterCache     capicontrollers.ClusterCache
	provClusterCache     provisioningcontrollers.ClusterCache
	mgmtClusterCache     mgmtcontrollers.ClusterCache
	rkeControlPlaneCache rkecontroller.RKEControlPlaneCache
	dynamic              *dynamic.Controller
}

func Register(ctx context.Context, clients *wrangler.Context) {
	h := handler{
		secrets:              clients.Core.Secret().Cache(),
		machines:             clients.CAPI.Machine(),
		bootstrapCache:       clients.RKE.RKEBootstrap().Cache(),
		bootstrapController:  clients.RKE.RKEBootstrap(),
		capiClusterCache:     clients.CAPI.Cluster().Cache(),
		mgmtClusterCache:     clients.Mgmt.Cluster().Cache(),
		provClusterCache:     clients.Provisioning.Cluster().Cache(),
		rkeControlPlaneCache: clients.RKE.RKEControlPlane().Cache(),
		dynamic:              clients.Dynamic,
	}
	clients.CAPI.Machine().OnChange(ctx, "machine-status", h.OnChange)

	relatedresource.Watch(ctx, "machine-trigger-from-secret", func(namespace, name string, obj runtime.Object) ([]relatedresource.Key, error) {
		if secret, ok := obj.(*corev1.Secret); ok {
			if secret.Type == planner.SecretTypeMachinePlan {
				return []relatedresource.Key{{
					Namespace: secret.Namespace,
					Name:      secret.Labels[planner.MachineNameLabel],
				}}, nil
			}
		}
		return nil, nil
	}, clients.CAPI.Machine(), clients.Core.Secret())

	h.dynamic.OnChange(ctx, "machine-trigger", func(gvk schema.GroupVersionKind) bool {
		return gvk.Group == "rke-machine.cattle.io"
	}, func(obj runtime.Object) (runtime.Object, error) {
		m, err := meta.Accessor(obj)
		if err != nil {
			return nil, err
		}
		for _, owner := range m.GetOwnerReferences() {
			if owner.Kind == "Machine" {
				h.machines.Enqueue(m.GetNamespace(), owner.Name)
			}
		}
		return obj, nil
	})
}

func (h *handler) setJoinURLFromOutput(machine *capi.Machine, nodePlan *plan.Node) error {
	if nodePlan == nil || !planner.IsEtcdOnlyInitNode(machine) || machine.Annotations[planner.JoinURLAnnotation] != "" {
		return nil
	}

	address, ok := nodePlan.Output["capture-address"]
	if !ok {
		return nil
	}

	var str string
	scanner := bufio.NewScanner(bytes.NewBuffer(address))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "{") {
			str = line
			break
		}
	}

	if str == "" {
		return nil
	}

	dbInfo := &dbinfo{}
	if err := json.Unmarshal([]byte(str), dbInfo); err != nil {
		return err
	}

	if len(dbInfo.Members) == 0 {
		return nil
	}

	if len(dbInfo.Members[0].ClientURLs) == 0 {
		return nil
	}

	u, err := url.Parse(dbInfo.Members[0].ClientURLs[0])
	if err != nil {
		return err
	}

	cluster, err := h.capiClusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
	if err != nil {
		return err
	}

	if cluster.Spec.ControlPlaneRef == nil {
		return nil
	}

	rkeControlPlane, err := h.rkeControlPlaneCache.Get(machine.Namespace, cluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		return err
	}

	joinURL := fmt.Sprintf("https://%s:%d", u.Hostname(),
		rancherruntime.GetRuntimeSupervisorPort(rkeControlPlane.Spec.KubernetesVersion))
	machine = machine.DeepCopy()
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}
	machine.Annotations[planner.JoinURLAnnotation] = joinURL
	_, err = h.machines.Update(machine)
	return err
}

func (h *handler) OnChange(_ string, machine *capi.Machine) (*capi.Machine, error) {
	if machine == nil ||
		machine.Spec.Bootstrap.ConfigRef == nil ||
		machine.Spec.Bootstrap.ConfigRef.Kind != "RKEBootstrap" {
		return machine, nil
	}

	status, err := h.getInfraMachineState(machine)
	if status.cond == InfrastructureReady {
		// If the machine is being deleted and the infrastructure machine object is not found or failed to delete,
		// then update the status of the machine object so the CAPI controller picks it up.
		return h.setMachineCondition(machine, status)
	} else if err != nil {
		return machine, err
	}

	rkeBootstrap, err := h.bootstrapCache.Get(machine.Namespace, machine.Spec.Bootstrap.ConfigRef.Name)
	if err != nil {
		if machine.DeletionTimestamp != nil && apierror.IsNotFound(err) {
			// If the machine is being deleted and the bootstrap object is not found,
			// then update the status of the machine object so the CAPI controller picks it up.
			return h.setMachineCondition(machine, &machineStatus{cond: BootstrapReady, status: corev1.ConditionFalse, reason: capi.DeletedReason, message: "bootstrap is deleted"})
		}
		return machine, err
	}

	secret, err := h.secrets.Get(machine.Namespace, planner.PlanSecretFromBootstrapName(rkeBootstrap.Name))
	if apierror.IsNotFound(err) {
		// When the secret exists this handler will be triggered, so don't error
		return machine, nil
	} else if err != nil {
		return machine, err
	}

	plan, err := planner.SecretToNode(secret)
	if err != nil {
		return machine, err
	}

	if err := h.setJoinURLFromOutput(machine, plan); err != nil {
		return machine, err
	}

	// This is a temporary solution until RKE2 Windows nodes support system-agent functionality.
	if os, ok := machine.GetLabels()["cattle.io/os"]; ok && os == "windows" {
		return h.setMachineCondition(machine, &machineStatus{cond: Provisioned, status: corev1.ConditionTrue, reason: "WindowsNode", message: "windows nodes don't currently support plans"})
	}

	if status.status == "" {
		status.status, status.reason, status.message = planner.GetPlanStatusReasonMessage(machine, plan)
	}

	if status.status == corev1.ConditionTrue && status.providerID == "" {
		status.status = corev1.ConditionUnknown
		status.reason = "NoProviderID"
		status.message = "waiting for node to be registered in Kubernetes"
		provCluster, err := h.provClusterCache.Get(machine.Namespace, machine.Spec.ClusterName)
		if err == nil {
			mgmtCluster, err := h.mgmtClusterCache.Get(provCluster.Status.ClusterName)
			if err == nil {
				if condition.Cond("Ready").IsTrue(mgmtCluster) {
					h.bootstrapController.Enqueue(machine.Spec.Bootstrap.ConfigRef.Namespace, machine.Spec.Bootstrap.ConfigRef.Name)
				} else if planner.IsOnlyEtcd(machine) {
					status.message = "waiting for cluster agent to be available on a control plane node"
					h.machines.EnqueueAfter(machine.Namespace, machine.Name, 2*time.Second)
				} else {
					status.message = "waiting for cluster agent to be available"
					h.machines.EnqueueAfter(machine.Namespace, machine.Name, 2*time.Second)
				}
			}
		}
	}

	return h.setMachineCondition(machine, status)
}

func (h *handler) setMachineCondition(machine *capi.Machine, status *machineStatus) (*capi.Machine, error) {
	if !status.machineStatusNeedsUpdate(machine) {
		return machine, nil
	}

	resetProvisioned := status.cond == InfrastructureReady
	machine = machine.DeepCopy()
	newCond := status.toCapiCondition()
	var set bool
	for i, c := range machine.Status.Conditions {
		if string(c.Type) == string(status.cond) {
			set = true
			machine.Status.Conditions[i] = newCond
		} else if resetProvisioned && string(c.Type) == string(Provisioned) && !Provisioned.IsTrue(machine) {
			// Ensure that the newCond status has precedence over the Provisioned condition
			machine.Status.Conditions[i] = capi.Condition{
				Type:               c.Type,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			}
		}
	}

	if !set {
		machine.Status.Conditions = append(machine.Status.Conditions, newCond)
	}

	if status.reason == capi.DeletionFailedReason {
		machine.Status.FailureReason = capierror.MachineStatusErrorPtr(capierror.MachineStatusError(status.reason))
		machine.Status.FailureMessage = &status.message
	}

	return h.machines.UpdateStatus(machine)
}

func (h *handler) getInfraMachineState(capiMachine *capi.Machine) (*machineStatus, error) {
	if capiMachine.DeletionTimestamp.IsZero() && capiMachine.Status.FailureReason != nil && capiMachine.Status.FailureMessage != nil {
		return &machineStatus{
			cond:    Provisioned,
			status:  corev1.ConditionFalse,
			reason:  string(capierror.CreateMachineError),
			message: fmt.Sprintf("failed creating server (%s) in infrastructure provider: %s: %s", capiMachine.Spec.InfrastructureRef.Kind, *capiMachine.Status.FailureReason, *capiMachine.Status.FailureMessage),
		}, nil
	}
	gvk := schema.FromAPIVersionAndKind(capiMachine.Spec.InfrastructureRef.APIVersion, capiMachine.Spec.InfrastructureRef.Kind)
	infraMachine, err := h.dynamic.Get(gvk, capiMachine.Namespace, capiMachine.Spec.InfrastructureRef.Name)
	if apierror.IsNotFound(err) {
		if !capiMachine.DeletionTimestamp.IsZero() {
			return &machineStatus{
				cond:    InfrastructureReady,
				status:  corev1.ConditionFalse,
				reason:  capi.DeletedReason,
				message: "machine infrastructure is deleted",
			}, nil
		}
		return &machineStatus{
			cond:    Provisioned,
			status:  corev1.ConditionUnknown,
			reason:  "NoMachineDefined",
			message: "waiting for machine to be defined",
		}, nil
	} else if err != nil {
		return nil, err
	}

	obj, err := data.Convert(infraMachine)
	if err != nil {
		return nil, err
	}

	if capiMachine.Spec.InfrastructureRef.APIVersion == "rke-machine.cattle.io/v1" {
		if capiMachine.DeletionTimestamp.IsZero() {
			if obj.String("status", "jobName") == "" {
				return &machineStatus{
					cond:    Provisioned,
					status:  corev1.ConditionUnknown,
					reason:  "NoJob",
					message: "waiting to schedule machine create",
				}, nil
			}

			if !obj.Bool("status", "jobComplete") {
				return &machineStatus{
					cond:    Provisioned,
					status:  corev1.ConditionUnknown,
					reason:  "Creating",
					message: fmt.Sprintf("creating server (%s) in infrastructure provider", capiMachine.Spec.InfrastructureRef.Kind),
				}, nil
			}
		} else {
			if obj.String("status", "failureReason") == string(capierror.DeleteMachineError) {
				return &machineStatus{
					cond:   InfrastructureReady,
					status: corev1.ConditionFalse,
					reason: capi.DeletionFailedReason,
					message: fmt.Sprintf("failed deleting server (%s) in infrastructure provider: %s: %s",
						capiMachine.Spec.InfrastructureRef.Kind,
						obj.String("status", "failureReason"),
						obj.String("status", "failureMessage"),
					),
					providerID: obj.String("spec", "providerID"),
				}, nil
			}

			return &machineStatus{
				cond:       InfrastructureReady,
				status:     corev1.ConditionUnknown,
				reason:     capi.DeletingReason,
				message:    fmt.Sprintf("deleting server (%s) in infrastructure provider", capiMachine.Spec.InfrastructureRef.Kind),
				providerID: obj.String("spec", "providerID"),
			}, nil
		}
	}

	return &machineStatus{cond: Provisioned, providerID: obj.String("spec", "providerID")}, nil
}

type dbinfo struct {
	Members []member `json:"members,omitempty"`
}
type member struct {
	ClientURLs []string `json:"clientURLs,omitempty"`
}
