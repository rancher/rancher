package secretrequest

import (
	"context"
	"fmt"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/apis/telemetry.cattle.io/v1"
	secretRequestControllers "github.com/rancher/rancher/pkg/generated/controllers/telemetry.cattle.io/v1"
	"github.com/rancher/rancher/pkg/telemetry"
	"github.com/rancher/rancher/pkg/telemetry/consts"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	//"k8s.io/apimachinery/pkg/api/errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const PayloadSecretKey = "payload"

type handler struct {
	ctx                context.Context
	secretRequests     secretRequestControllers.SecretRequestController
	secretRequestCache secretRequestControllers.SecretRequestCache

	// needed for secret creation
	systemProject *mgmtv3.Project
	namespaces    v1core.NamespaceController
	secrets       v1core.SecretController // TODO: maybe add secret cache?

	telemetryManager telemetry.TelemetryExporterManager
}

func Register(
	ctx context.Context,
	secretRequests secretRequestControllers.SecretRequestController,
	secretRequestCache secretRequestControllers.SecretRequestCache,
	systemProject *mgmtv3.Project,
	namespaces v1core.NamespaceController,
	secrets v1core.SecretController,
	telemetryManager telemetry.TelemetryExporterManager,
) {
	controller := &handler{
		ctx:                ctx,
		secretRequests:     secretRequests,
		secretRequestCache: secretRequestCache,
		systemProject:      systemProject,
		namespaces:         namespaces,
		secrets:            secrets,
		telemetryManager:   telemetryManager,
	}

	secretRequests.OnChange(ctx, "secret-requests", controller.OnSecretRequestChange)
	secretRequests.OnRemove(ctx, "secret-requests", controller.OnSecretRequestRemove)
}

func exporterIdFromObj(req *v1.SecretRequest) string {
	return fmt.Sprintf("scc-%s-%s", req.Namespace, req.Name)
}

func (h *handler) OnSecretRequestRemove(key string, incomingObj *v1.SecretRequest) (*v1.SecretRequest, error) {
	if incomingObj == nil {
		return nil, nil
	}
	h.telemetryManager.Delete(exporterIdFromObj(incomingObj))
	return nil, nil
}

// FIXME: status and retry considerations should be more carefully implemented here
func (h *handler) OnSecretRequestChange(key string, incomingObj *v1.SecretRequest) (*v1.SecretRequest, error) {
	if incomingObj == nil {
		return nil, nil
	}
	logrus.Debugf("Received secret request for '%s'", incomingObj.Name)
	logrus.Debug(incomingObj)

	if !incomingObj.HasCondition(v1.ResourceConditionProgressing) {
		preparedObj := incomingObj.DeepCopy()
		v1.ResourceConditionProgressing.True(preparedObj)
		_, err := h.secretRequests.UpdateStatus(preparedObj)

		return incomingObj, err
	}

	if isValid, err := h.isValidSecretRequest(incomingObj); !isValid {
		preparedObj := incomingObj.DeepCopy()
		v1.ResourceConditionFailure.SetError(preparedObj, "Secret Request is in an invalid state", err)
		v1.ResourceConditionFailure.True(preparedObj)
		v1.ResourceConditionProgressing.False(preparedObj)
		v1.ResourceConditionDone.True(preparedObj)
		_, updateErr := h.secretRequests.UpdateStatus(preparedObj)

		if updateErr != nil {
			newErr := fmt.Errorf("error validating secret request: %w; and additional error updating secret request: %w", err, updateErr)
			return nil, newErr
		}

		return incomingObj, err
	}

	uniqueName := exporterIdFromObj(incomingObj)
	if h.telemetryManager.Has(uniqueName) {
		h.telemetryManager.Delete(uniqueName)
	}

	h.telemetryManager.Register(
		uniqueName,
		telemetry.NewSecretExporter(
			h.secrets,
			&corev1.SecretReference{
				Name:      incomingObj.Spec.TargetSecretRef.Name,
				Namespace: incomingObj.Spec.TargetSecretRef.Namespace,
			},
		),
		time.Second*60,
	)
	preparedObj := incomingObj.DeepCopy()
	v1.ResourceConditionDone.True(preparedObj)
	v1.ResourceConditionProgressing.False(preparedObj)
	preparedObj.RemoveCondition(v1.ResourceConditionFailure)
	timeNow := metav1.Now()
	preparedObj.Status.LastSyncTS = &timeNow
	updated, updateErr := h.secretRequests.UpdateStatus(preparedObj)
	if updateErr != nil {
		return incomingObj, fmt.Errorf("error updating secret request '%s': %w", incomingObj.Spec.TargetSecretRef, updateErr)
	}
	logrus.Debugf("Updated secret request '%v'", updated)
	return incomingObj, nil
}

func (h *handler) isValidSecretRequest(secretRequest *v1.SecretRequest) (bool, error) {
	if secretRequest == nil {
		return false, fmt.Errorf("secretRequest cannot be nil")
	}

	// could design an enum system here, but I think it's fine
	if secretRequest.Spec.SecretType != "scc" {
		return false, fmt.Errorf("secretRequest.Spec.SecretType must be 'scc'")
	}

	if secretRequest.Spec.TargetSecretRef == nil {
		return false, fmt.Errorf("secretRequest.Spec.TargetSecretRef cannot be nil")
	}

	if secretRequest.Spec.TargetSecretRef.Name == "" || secretRequest.Spec.TargetSecretRef.Namespace == "" {
		return false, fmt.Errorf("secretRequest.Spec.TargetSecretRef.Namespace and secretRequest.Spec.TargetSecretRef.Name cannot be empty")
	}

	targetNamespace, err := h.namespaces.Get(secretRequest.Spec.TargetSecretRef.Namespace, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("secretRequest.Spec.TargetSecretRef.Namespace must be a real namespace: %w", err)
	}

	targetNamespaceProjectLabel := targetNamespace.Labels[consts.ProjectFieldKey]
	if targetNamespaceProjectLabel != h.systemProject.Name {
		return false, fmt.Errorf("secretRequest.Spec.TargetSecretRef.Namespace is not within the 'System' Project")
	}

	return true, nil
}
