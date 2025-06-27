package auditpolicy

import (
	"context"
	"fmt"

	auditlogv1 "github.com/rancher/rancher/pkg/apis/auditlog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/auth/audit"
	"github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/auditlog.cattle.io/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	reasonPolicyNotYetActivated = "PolicyNotYetActivated"
	reasonPolicyIsActive        = "PolicyIsActive"
	reasonPolicyIsInvalid       = "PolicyIsInvalid"
	reasonPolicyWasDisabled     = "PolicyWasDisabled"
)

type handler struct {
	auditpolicy v1.AuditPolicyController
	writer      *audit.Writer

	time func() metav1.Time
}

func (h *handler) OnChange(key string, obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if obj == nil {
		return obj, nil
	}

	if meta.FindStatusCondition(obj.Status.Conditions, auditlogv1.AuditPolicyConditionTypeActive) == nil {
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               string(auditlogv1.AuditPolicyConditionTypeActive),
			Status:             metav1.ConditionUnknown,
			ObservedGeneration: obj.GetGeneration(),
			LastTransitionTime: h.time(),
			Reason:             reasonPolicyNotYetActivated,
		})
	}

	var err error

	if !obj.Spec.Enabled {
		h.writer.RemovePolicy(obj)

		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               auditlogv1.AuditPolicyConditionTypeActive,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: h.time(),
			Reason:             reasonPolicyWasDisabled,
		})

		if obj, err = h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy disabled: %s", err)
		}

		return obj, nil
	}

	if err := h.writer.UpdatePolicy(obj); err != nil {
		meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
			Type:               auditlogv1.AuditPolicyConditionTypeActive,
			Status:             metav1.ConditionFalse,
			LastTransitionTime: h.time(),
			Reason:             reasonPolicyIsInvalid,
			Message:            err.Error(),
		})

		if obj, err = h.auditpolicy.UpdateStatus(obj); err != nil {
			return obj, fmt.Errorf("could not mark audit log policy invalid: %s", err)
		}

		return obj, nil
	}

	meta.SetStatusCondition(&obj.Status.Conditions, metav1.Condition{
		Type:               auditlogv1.AuditPolicyConditionTypeActive,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: h.time(),
		Reason:             reasonPolicyIsActive,
	})

	if obj, err := h.auditpolicy.UpdateStatus(obj); err != nil {
		return obj, fmt.Errorf("could not mark audit log policy active: %s", err)
	}

	return obj, nil
}

func (h *handler) OnRemove(key string, obj *auditlogv1.AuditPolicy) (*auditlogv1.AuditPolicy, error) {
	if obj == nil {
		return obj, nil
	}

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
