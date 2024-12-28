package auditlogpolicy

import (
	"context"
	"fmt"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

type handler struct {
	auditlogpolicy v1.AuditLogPolicyController
	writer         *audit.Writer
}

func (h *handler) OnChange(key string, obj *auditlogv1.AuditLogPolicy) (*auditlogv1.AuditLogPolicy, error) {
	if !obj.Spec.Enabled {
		h.writer.RemovePolicy(obj)

		obj.Status = auditlogv1.AuditLogPolicyStatus{
			Condition: auditlogv1.AuditLogPolicyStatusConditionDisabled,
		}

		if _, err := h.auditlogpolicy.UpdateStatus(obj); err != nil {
			logrus.Errorf("could not mark audit log policy '%s/%s' as disabled: %s", obj.Namespace, obj.Name, err)
			return obj, err
		}

		return obj, nil
	}

	if err := h.writer.UpdatePolicy(obj); err != nil {
		obj.Status = auditlogv1.AuditLogPolicyStatus{
			Condition: auditlogv1.AuditLogPolicyStatusConditionInvalid,
			Message:   err.Error(),
		}

		if _, err := h.auditlogpolicy.UpdateStatus(obj); err != nil {
			logrus.Errorf("could not mark audit log policy '%s/%s' as invalid: %s", obj.Namespace, obj.Name, err)
			return obj, err
		}

		return obj, nil
	}

	obj.Status.Condition = auditlogv1.AuditLogPolicyStatusConditionActive
	obj.Status.Message = ""
	if _, err := h.auditlogpolicy.UpdateStatus(obj); err != nil {
		logrus.Errorf("could not mark audit log policy '%s/%s' as active: %s", obj.Namespace, obj.Name, err)
		return obj, err
	}

	return obj, nil
}

func (h *handler) OnRemove(key string, obj *auditlogv1.AuditLogPolicy) (*auditlogv1.AuditLogPolicy, error) {
	if obj.Spec.Enabled {
		if ok := h.writer.RemovePolicy(obj); !ok {
			return obj, fmt.Errorf("failed to remove policy '%s/%s' from writer", obj.Namespace, obj.Name)
		}
	}

	return obj, nil
}

func Register(ctx context.Context, writer *audit.Writer, controller auditlog.Interface) error {
	h := &handler{
		auditlogpolicy: controller.V1().AuditLogPolicy(),
		writer:         writer,
	}

	controller.V1().AuditLogPolicy().OnChange(ctx, "auditlog-policy-controller", h.OnChange)
	controller.V1().AuditLogPolicy().OnRemove(ctx, "auditlog-policy-controller-remover", h.OnRemove)

	return nil
}
