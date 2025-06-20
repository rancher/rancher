package offline

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/SUSE/connect-ng/pkg/registration"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (o *SecretManager) SetRegistrationOfflineCertificateSecretRef(registrationObj *v1.Registration) *v1.Registration {
	registrationObj.Spec.OfflineRegistrationCertificateSecretRef = &corev1.SecretReference{
		Namespace: o.secretNamespace,
		Name:      o.certificateSecretName,
	}
	return registrationObj
}

func (o *SecretManager) loadCertificateSecret() ([]byte, error) {
	var currentOfflineCert []byte
	offlineCert, err := o.secretCache.Get(o.secretNamespace, o.certificateSecretName)
	if err != nil || offlineCert == nil {
		return nil, fmt.Errorf("error loading certificate secret: %v", err)
	}

	if len(offlineCert.Data) == 0 {
		return nil, fmt.Errorf("secret %s/%s has no data fields; but should always have them", o.secretNamespace, o.certificateSecretName)
	}
	var certOk bool
	currentOfflineCert, certOk = offlineCert.Data[consts.SecretKeyOfflineRegCert]
	if !certOk {
		return nil, fmt.Errorf("secret %s/%s has no data field for %s", o.secretNamespace, o.certificateSecretName, consts.SecretKeyOfflineRegRequest)
	}

	return currentOfflineCert, nil
}

func (o *SecretManager) InitCertificateSecret() error {
	return o.saveCertificateSecret()
}

func (o *SecretManager) saveCertificateSecret() error {
	create := false
	// TODO gather errors
	offlineRequest, err := o.secrets.Get(o.secretNamespace, o.certificateSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		create = true
		offlineRequest = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.certificateSecretName,
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
	labels[consts.LabelSccSecretRole] = string(consts.OfflineCertificate)
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
		curOfflineRequest, err := o.secrets.Get(o.secretNamespace, o.certificateSecretName, metav1.GetOptions{})
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

func (o *SecretManager) UpdateOfflineCertificate(inReq *registration.OfflineRequest) error {
	jsonOfflineRequest, err := json.Marshal(inReq)
	if err != nil {
		return err
	}

	// TODO: get sha of request/secret data then compare to see if actually needs update?
	o.offlineRequest = jsonOfflineRequest

	return o.saveCertificateSecret()
}

func (o *SecretManager) OfflineCertificateReader() (io.Reader, error) {
	certBytes, err := o.loadCertificateSecret()
	if err != nil {
		return nil, err
	}

	encodedCertBytes := make([]byte, base64.StdEncoding.EncodedLen(len(certBytes)))
	// TODO remove after SCC library update
	base64.StdEncoding.Encode(encodedCertBytes, certBytes)

	reader := bytes.NewReader(encodedCertBytes)

	return reader, nil
}

func (o *SecretManager) RemoveOfflineCertificate() error {
	delErr := o.secrets.Delete(o.secretNamespace, o.certificateSecretName, &metav1.DeleteOptions{})
	if delErr != nil && apierrors.IsNotFound(delErr) {
		return nil
	}
	return delErr
}
