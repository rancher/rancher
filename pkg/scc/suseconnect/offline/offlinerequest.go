package offline

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/SUSE/connect-ng/pkg/registration"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (o *SecretManager) SetRegistrationOfflineRegistrationRequestSecretRef(registrationObj *v1.Registration) *v1.Registration {
	registrationObj.Status.OfflineRegistrationRequest = &corev1.SecretReference{
		Namespace: o.secretNamespace,
		Name:      o.requestSecretName,
	}
	return registrationObj
}

func (o *SecretManager) loadRequestSecret() error {
	offlineRequest, err := o.secretCache.Get(o.secretNamespace, o.requestSecretName)
	if err == nil && offlineRequest != nil {
		if len(offlineRequest.Data) == 0 {
			return fmt.Errorf("secret %s/%s has no data fields; but should always have them", o.secretNamespace, o.requestSecretName)
		}
		currentOfflineRequest, ok := offlineRequest.Data[consts.SecretKeyOfflineRegRequest]
		if !ok {
			return fmt.Errorf("secret %s/%s has no data field for %s", o.secretNamespace, o.requestSecretName, consts.SecretKeyOfflineRegRequest)
		}

		o.offlineRequest = currentOfflineRequest
	}

	return nil
}

func (o *SecretManager) InitRequestSecret() error {
	return o.saveRequestSecret()
}

func (o *SecretManager) saveRequestSecret() error {
	create := false
	// TODO gather errors
	offlineRequest, err := o.secrets.Get(o.secretNamespace, o.requestSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		create = true
		offlineRequest = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.requestSecretName,
				Namespace: o.secretNamespace,
			},
		}
	}

	if offlineRequest.Data == nil {
		offlineRequest.Data = map[string][]byte{
			consts.SecretKeyOfflineRegRequest: make([]byte, 0),
		}
	}

	if len(o.offlineRequest) != 0 {
		offlineRequest.Data[consts.SecretKeyOfflineRegRequest] = o.offlineRequest
	}

	if o.finalizer != "" {
		if offlineRequest.Finalizers == nil {
			offlineRequest.Finalizers = []string{}
		}
		if !slices.Contains(offlineRequest.Finalizers, o.finalizer) {
			offlineRequest.Finalizers = append(offlineRequest.Finalizers, o.finalizer)
		}
	}

	labels := o.defaultLabels
	labels[consts.LabelSccSecretRole] = string(consts.OfflineRequestRole)
	if offlineRequest.Labels == nil {
		offlineRequest.Labels = labels
	} else {
		maps.Copy(offlineRequest.Labels, labels)
	}

	if o.ownerRef != nil {
		offlineRequest.OwnerReferences = []metav1.OwnerReference{*o.ownerRef}
	}

	var createOrUpdateErr error
	if create {
		_, createOrUpdateErr = o.secrets.Create(offlineRequest)
	} else {
		// TODO(alex): this was a hack that makes it work...which makes me think secretCache is root of issue?
		curOfflineRequest, err := o.secrets.Get(o.secretNamespace, o.requestSecretName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		prepared := curOfflineRequest.DeepCopy()
		prepared.Data = offlineRequest.Data
		prepared.OwnerReferences = offlineRequest.OwnerReferences
		prepared.Finalizers = offlineRequest.Finalizers
		prepared.Labels = offlineRequest.Labels

		_, createOrUpdateErr = o.secrets.Update(prepared)
	}

	return createOrUpdateErr
}

func (o *SecretManager) UpdateOfflineRequest(inReq *registration.OfflineRequest) error {
	jsonOfflineRequest, err := json.Marshal(inReq)
	if err != nil {
		return err
	}

	// TODO: get sha of request/secret data then compare to see if actually needs update?
	o.offlineRequest = jsonOfflineRequest

	return o.saveRequestSecret()
}

func (o *SecretManager) RemoveOfflineRequest() error {
	delErr := o.secrets.Delete(o.secretNamespace, o.requestSecretName, &metav1.DeleteOptions{})
	if delErr != nil && apierrors.IsNotFound(delErr) {
		return nil
	}
	return delErr
}
