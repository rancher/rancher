package offlinerequest

import (
	"fmt"
	"github.com/SUSE/connect-ng/pkg/registration"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	v1core "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"io"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"slices"
)

const (
	RequestKey = "request"
)

type OfflineRegistrationSecrets struct {
	secretNamespace string
	secretName      string
	finalizer       string
	ownerRef        *metav1.OwnerReference
	secrets         v1core.SecretController
	secretCache     v1core.SecretCache
	offlineRequest  []byte
}

func New(
	namespace, name string,
	finalizer string,
	ownerRef *metav1.OwnerReference,
	secrets v1core.SecretController,
	secretCache v1core.SecretCache,
) *OfflineRegistrationSecrets {
	return &OfflineRegistrationSecrets{
		secretNamespace: namespace,
		secretName:      name,
		secrets:         secrets,
		secretCache:     secretCache,
		finalizer:       finalizer,
		ownerRef:        ownerRef,
	}
}

func (o *OfflineRegistrationSecrets) SetRegistrationOfflineRegistrationRequestSecretRef(registrationObj *v1.Registration) *v1.Registration {
	registrationObj.Status.OfflineRegistrationRequest = &corev1.SecretReference{
		Namespace: o.secretNamespace,
		Name:      o.secretName,
	}
	return registrationObj
}

func (o *OfflineRegistrationSecrets) loadSecret() error {
	offlineRequest, err := o.secretCache.Get(o.secretNamespace, o.secretName)
	if err == nil && offlineRequest != nil {
		if len(offlineRequest.Data) == 0 {
			return fmt.Errorf("secret %s/%s has no data fields; but should always have them", o.secretNamespace, o.secretName)
		}
		currentOfflineRequest, ok := offlineRequest.Data[RequestKey]
		if !ok {
			return fmt.Errorf("secret %s/%s has no data field for %s", o.secretNamespace, o.secretName, RequestKey)
		}

		o.offlineRequest = currentOfflineRequest
	}

	return nil
}

func (o *OfflineRegistrationSecrets) InitSecret() error {
	return o.saveSecret()
}

func (o *OfflineRegistrationSecrets) saveSecret() error {
	create := false
	// TODO gather errors
	sccCreds, err := o.secretCache.Get(o.secretNamespace, o.secretName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		create = true
		sccCreds = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.secretName,
				Namespace: o.secretNamespace,
			},
		}
	}

	if sccCreds.Data == nil {
		sccCreds.Data = map[string][]byte{
			RequestKey: make([]byte, 0),
		}
	}

	if len(o.offlineRequest) != 0 {
		sccCreds.Data[RequestKey] = o.offlineRequest
	}

	if o.finalizer != "" {
		if sccCreds.Finalizers == nil {
			sccCreds.Finalizers = []string{}
		}
		if !slices.Contains(sccCreds.Finalizers, o.finalizer) {
			sccCreds.Finalizers = append(sccCreds.Finalizers, o.finalizer)
		}
	}

	if o.ownerRef != nil {
		sccCreds.OwnerReferences = []metav1.OwnerReference{*o.ownerRef}
	}

	var createOrUpdateErr error
	if create {
		_, createOrUpdateErr = o.secrets.Create(sccCreds)
	} else {
		// TODO(alex): this was a hack that makes it work...which makes me think secretCache is root of issue?
		curSccCreds, err := o.secrets.Get(o.secretNamespace, o.secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		prepared := curSccCreds.DeepCopy()
		prepared.Data = sccCreds.Data
		prepared.OwnerReferences = sccCreds.OwnerReferences
		prepared.Finalizers = sccCreds.Finalizers
		prepared.Labels = sccCreds.Labels

		_, createOrUpdateErr = o.secrets.Update(prepared)
	}

	return createOrUpdateErr
}

func (o *OfflineRegistrationSecrets) UpdateOfflineRequest(inReq *registration.OfflineRequest) error {
	base64StringReader, err := inReq.Base64Encoded()
	if err != nil {
		return err
	}

	var orrBytes []byte
	orrBytes, err = io.ReadAll(base64StringReader)
	if err != nil {
		return err
	}

	// TODO: get sha of request/secret data then compare to see if actually needs update?
	o.offlineRequest = orrBytes

	return o.saveSecret()
}
