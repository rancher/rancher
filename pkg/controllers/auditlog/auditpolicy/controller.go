package auditpolicy

import (
	"context"
	"fmt"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io/v1"
)

type handler struct {
	auditpolicy v1.AuditPolicyController
	writer      *audit.Writer
}

func (h *handler) OnChange(key string, obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if !obj.Spec.Enabled {
		h.writer.RemovePolicy(obj)

		obj.Status = auditlogv1.AuditPolicyStatus{
			Condition: auditlogv1.AuditPolicyStatusConditionInactive,
		}

		if _, err := h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy '%s/%s' as disabled: %s", obj.Namespace, obj.Name, err)
		}

		return obj, nil
	}

	if err := h.writer.UpdatePolicy(obj); err != nil {
		obj.Status = auditlogv1.AuditPolicyStatus{
			Condition: auditlogv1.AuditPolicyStatusConditionInvalid,
			Message:   err.Error(),
		}

		if _, err := h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy '%s/%s' as invalid: %s", obj.Namespace, obj.Name, err)
		}

		return obj, nil
	}

	obj.Status.Condition = auditlogv1.AuditPolicyStatusConditionActive
	obj.Status.Message = ""
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
