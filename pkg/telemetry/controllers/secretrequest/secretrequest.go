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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
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
) {
	controller := &handler{
		ctx:                ctx,
		secretRequests:     secretRequests,
		secretRequestCache: secretRequestCache,
		systemProject:      systemProject,
		namespaces:         namespaces,
		secrets:            secrets,
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

func (h *handler) OnSecretRequestChange(key string, incomingObj *v1.SecretRequest) (*v1.SecretRequest, error) {
	if incomingObj == nil {
		return nil, nil
	}

	logrus.Debugf("Received secret request for '%s'", incomingObj.Name)
	logrus.Debug(incomingObj)

	// TODO: does this need to verify we're going to make changes somehow?
	if !incomingObj.HasCondition(v1.ResourceConditionProgressing) {
		// TODO: in retry loop?
		preparedObj := incomingObj.DeepCopy()
		v1.ResourceConditionProgressing.True(preparedObj)
		_, err := h.secretRequests.UpdateStatus(preparedObj)

		return incomingObj, err
	}

	if isValid, err := h.isValidSecretRequest(incomingObj); !isValid {
		// TODO: in retry loop?
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
	//existingTargetSecret, existingErr := h.secrets.Get(incomingObj.Spec.TargetSecretRef.Namespace, incomingObj.Spec.TargetSecretRef.Name, metav1.GetOptions{})
	//if existingErr != nil && !errors.IsNotFound(existingErr) {
	//	logrus.Errorf("error getting existing target secret for secret request '%s': %v", incomingObj.Name, existingErr)
	//	return incomingObj, existingErr
	//}

	// TODO: something to actually prepare the secret data (or fetch it from the main instance of the secret of that type to clone it to this target)?

	//desiredSecret := &corev1.Secret{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name:      incomingObj.Spec.TargetSecretRef.Name,
	//		Namespace: incomingObj.Spec.TargetSecretRef.Namespace,
	//	},
	//	Data: map[string][]byte{
	//		PayloadSecretKey: []byte("TODO: some secret data"),
	//	},
	//}
	//
	//if existingErr != nil && errors.IsNotFound(existingErr) {
	//	created, createErr := h.secrets.Create(desiredSecret)
	//	if createErr != nil {
	//		return nil, fmt.Errorf("error creating target secret '%s': %w", incomingObj.Spec.TargetSecretRef, createErr)
	//	}
	//	logrus.Debugf("Created target secret '%s'", created)
	//} else {
	//	preparedSecret := existingTargetSecret.DeepCopy()
	//	preparedSecret.Data = desiredSecret.Data
	//	updated, updateErr := h.secrets.Update(preparedSecret)
	//	if updateErr != nil {
	//		return nil, fmt.Errorf("error updating target secret '%s': %w", incomingObj.Spec.TargetSecretRef, updateErr)
	//	}
	//	logrus.Debugf("Updated target secret '%s'", updated)
	//}
	// TODO: (dan)
	preparedObj := incomingObj.DeepCopy()
	v1.ResourceConditionDone.True(preparedObj)
	v1.ResourceConditionProgressing.False(preparedObj)
	timeNow := metav1.Now()
	preparedObj.Status.LastSyncTS = &timeNow
	updated, updateErr := h.secretRequests.UpdateStatus(preparedObj)
	if updateErr != nil {
		return incomingObj, fmt.Errorf("error updating secret request '%s': %w", incomingObj.Spec.TargetSecretRef, updateErr)
	}

	logrus.Debugf("Updated secret request '%s'", updated)
	return incomingObj, nil
}

func (h *handler) isValidSecretRequest(secretRequest *v1.SecretRequest) (bool, error) {
	if secretRequest == nil {
		return false, fmt.Errorf("secretRequest cannot be nil")
	}

	// TODO: better way to manage this enums maybe?
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
