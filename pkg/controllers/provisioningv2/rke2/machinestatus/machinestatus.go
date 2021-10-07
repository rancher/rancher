package machinestatus

import (
	"fmt"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2/machineprovision"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/data"
	corev1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
	capierror "sigs.k8s.io/cluster-api/errors"
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
					message: machineprovision.CreatingMachineMessage(capiMachine.Spec.InfrastructureRef.Kind),
				}, nil
			}
		} else {
			if obj.String("status", "failureReason") == string(capierror.DeleteMachineError) {
				return &machineStatus{
					cond:       InfrastructureReady,
					status:     corev1.ConditionFalse,
					reason:     obj.String("status", "failureReason"),
					message:    machineprovision.FailedMachineDeleteMessage(capiMachine.Spec.InfrastructureRef.Kind, obj.String("status", "failureReason"), obj.String("status", "failureMessage")),
					providerID: obj.String("spec", "providerID"),
				}, nil
			}

			return &machineStatus{
				cond:       InfrastructureReady,
				status:     corev1.ConditionFalse,
				reason:     capi.DeletingReason,
				message:    machineprovision.DeletingMachineMessage(capiMachine.Spec.InfrastructureRef.Kind),
				providerID: obj.String("spec", "providerID"),
			}, nil
		}
	}

	return &machineStatus{cond: Provisioned, providerID: obj.String("spec", "providerID")}, nil
}
