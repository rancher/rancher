package offline

import (
	"bytes"
	"fmt"
	"io"
	"maps"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/controllers/common"
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
	offlineCert, err := o.secrets.Get(o.secretNamespace, o.certificateSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		create = true
		offlineCert = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.certificateSecretName,
				Namespace: o.secretNamespace,
			},
		}
	}

	if offlineCert.Data == nil {
		offlineCert.Data = map[string][]byte{
			consts.SecretKeyOfflineRegRequest: make([]byte, 0),
		}
	}

	if len(o.offlineRequest) != 0 {
		offlineCert.Data[consts.SecretKeyOfflineRegRequest] = o.offlineRequest
	}

	offlineCert = common.SecretAddOfflineFinalizer(offlineCert)

	labels := o.defaultLabels
	if offlineCert.Labels == nil {
		offlineCert.Labels = labels
	} else {
		maps.Copy(offlineCert.Labels, labels)
	}

	if o.ownerRef != nil {
		offlineCert.OwnerReferences = []metav1.OwnerReference{*o.ownerRef}
	}

	var createOrUpdateErr error
	if create {
		_, createOrUpdateErr = o.secrets.Create(offlineCert)
	} else {
		// TODO(alex): this was a hack that makes it work...which makes me think secretCache is root of issue?
		curOfflineRequest, err := o.secrets.Get(o.secretNamespace, o.certificateSecretName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		prepared := curOfflineRequest.DeepCopy()
		prepared.Data = offlineCert.Data
		prepared.OwnerReferences = offlineCert.OwnerReferences
		prepared.Finalizers = offlineCert.Finalizers
		prepared.Labels = offlineCert.Labels

		_, createOrUpdateErr = o.secrets.Update(prepared)
	}

	return createOrUpdateErr
}

func (o *SecretManager) OfflineCertificateReader() (io.Reader, error) {
	certBytes, err := o.loadCertificateSecret()
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(certBytes)

	return reader, nil
}

func (o *SecretManager) RemoveOfflineCertificate() error {
	currentSecret, err := o.secretCache.Get(o.secretNamespace, o.certificateSecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if removeFinalizerErr := o.removeOfflineFinalizer(currentSecret); removeFinalizerErr != nil {
		return removeFinalizerErr
	}

	delErr := o.secrets.Delete(o.secretNamespace, o.certificateSecretName, &metav1.DeleteOptions{})
	if delErr != nil && apierrors.IsNotFound(delErr) {
		return nil
	}
	return delErr
}
