package auditpolicy

import (
	"context"
	"fmt"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func updateStatus(obj *auditlogv1.AuditPolicy, condition auditlogv1.AuditPolicyConditionType, status metav1.ConditionStatus, message string) {
	for i, cond := range obj.Status.Conditions {
		if cond.Type == string(condition) {
			obj.Status.Conditions[i] = metav1.Condition{
				Type:               string(condition),
				Status:             status,
				ObservedGeneration: obj.GetGeneration(),
				LastTransitionTime: metav1.Now(),
				Message:            message,
			}

			break
		}
	}
}

type handler struct {
	auditpolicy v1.AuditPolicyController
	writer      *audit.Writer
}

func (h *handler) OnChange(key string, obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if len(obj.Status.Conditions) == 0 {
		obj.Status.Conditions = []metav1.Condition{
			{
				Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
				Status:             metav1.ConditionUnknown,
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: obj.GetGeneration(),
			},
			{
				Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
				Status:             metav1.ConditionUnknown,
				LastTransitionTime: metav1.Now(),
				ObservedGeneration: obj.GetGeneration(),
			},
		}
	}

	if !obj.Spec.Enabled {
		h.writer.RemovePolicy(obj)

		updateStatus(obj, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "policy was disabled")

		if _, err := h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy '%s/%s' as disabled: %s", obj.Namespace, obj.Name, err)
		}

		return obj, nil
	}

	if err := h.writer.UpdatePolicy(obj); err != nil {
		updateStatus(obj, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, err.Error())

		if _, err := h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy '%s/%s' as invalid: %s", obj.Namespace, obj.Name, err)
		}

		return obj, nil
	}

	updateStatus(obj, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")
	updateStatus(obj, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionTrue, "")

	if _, err := h.auditpolicy.UpdateStatus(obj); err != nil {
		return obj, fmt.Errorf("could not mark audit log policy '%s/%s' as active: %s", obj.Namespace, obj.Name, err)
	}

	return obj, nil
}

func (h *handler) OnRemove(key string, obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if obj.Spec.Enabled {
		if ok := h.writer.RemovePolicy(obj); !ok {
			return obj, fmt.Errorf("failed to remove policy '%s/%s' from writer", obj.Namespace, obj.Name)
		}
	}

	return obj, nil
}

func Register(ctx context.Context, writer *audit.Writer, controller auditlog.Interface) error {
	h := &handler{
		auditpolicy: controller.V1().AuditPolicy(),
		writer:      writer,
	}

	controller.V1().AuditPolicy().OnChange(ctx, "auditlog-policy-controller", h.OnChange)
	controller.V1().AuditPolicy().OnRemove(ctx, "auditlog-policy-controller-remover", h.OnRemove)

	return nil
}
