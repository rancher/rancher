package auditpolicy

import (
	"context"
	"fmt"
	"slices"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func hasCondition(obj *auditlogv1.AuditPolicy, t auditlogv1.AuditPolicyConditionType) bool {
	return slices.ContainsFunc(obj.Status.Conditions, func(c metav1.Condition) bool {
		return c.Type == string(t)
	})
}

type handler struct {
	auditpolicy v1.AuditPolicyController
	writer      *audit.Writer

	time func() metav1.Time
}

func (h *handler) updateStatus(obj *auditlogv1.AuditPolicy, condition auditlogv1.AuditPolicyConditionType, status metav1.ConditionStatus, message string) {
	for i, cond := range obj.Status.Conditions {
		if cond.Type == string(condition) {
			obj.Status.Conditions[i] = metav1.Condition{
				Type:               string(condition),
				Status:             status,
				ObservedGeneration: obj.GetGeneration(),
				LastTransitionTime: h.time(),
				Message:            message,
			}

			return
		}
	}
}

func (h *handler) OnChange(key string, obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if !hasCondition(obj, auditlogv1.AuditPolicyConditionTypeActive) {
		obj.Status.Conditions = append(obj.Status.Conditions, metav1.Condition{
			Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: obj.GetGeneration(),
			LastTransitionTime: h.time(),
		})
	}

	if !hasCondition(obj, auditlogv1.AuditPolicyConditionTypeValid) {
		obj.Status.Conditions = append(obj.Status.Conditions, metav1.Condition{
			Type:               string(auditlogv1.AuditPolicyConditionTypeValid),
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: obj.GetGeneration(),
			LastTransitionTime: h.time(),
		})
	}

	var err error

	if !obj.Spec.Enabled {
		h.writer.RemovePolicy(obj)

		h.updateStatus(obj, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionFalse, "policy was disabled")

		if obj, err = h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy disabled: %s", err)
		}

		return obj, nil
	}

	if err := h.writer.UpdatePolicy(obj); err != nil {
		h.updateStatus(obj, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionFalse, err.Error())

		if obj, err = h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy invalid: %s", err)
		}

		return obj, nil
	}

	h.updateStatus(obj, auditlogv1.AuditPolicyConditionTypeActive, metav1.ConditionTrue, "")
	h.updateStatus(obj, auditlogv1.AuditPolicyConditionTypeValid, metav1.ConditionTrue, "")

	if obj, err := h.auditpolicy.UpdateStatus(obj); err != nil {
		return obj, fmt.Errorf("could not mark audit log policy active: %s", err)
	}

	return obj, nil
}

func (h *handler) OnRemove(key string, obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if obj.Spec.Enabled {
		if ok := h.writer.RemovePolicy(obj); !ok {
			return obj, fmt.Errorf("failed to remove policy '%s' from writer", obj.Name)
		}
	}

	return obj, nil
}

func Register(ctx context.Context, writer *audit.Writer, controller auditlog.Interface) error {
	h := &handler{
		auditpolicy: controller.V1().AuditPolicy(),
		writer:      writer,

		time: func() metav1.Time {
			return metav1.Now()
		},
	}

	controller.V1().AuditPolicy().OnChange(ctx, "auditlog-policy-controller", h.OnChange)
	controller.V1().AuditPolicy().OnRemove(ctx, "auditlog-policy-controller-remover", h.OnRemove)

	return nil
}
