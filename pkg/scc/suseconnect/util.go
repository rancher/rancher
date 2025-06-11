package suseconnect

import (
	"fmt"
	"github.com/rancher/rancher/pkg/scc/util"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func FetchSccRegistrationCodeFrom(secrets controllerv1.SecretController, reference *corev1.SecretReference) string {
	sccContextLogger().Debugf("Fetching SCC Registration Code from secret %s/%s", reference.Namespace, reference.Name)
	regSecret, err := secrets.Get(reference.Namespace, reference.Name, metav1.GetOptions{})
	if err != nil {
		sccContextLogger().Warnf("Failed to get SCC Registration Code from secret %s/%s: %v", reference.Namespace, reference.Name, err)
		return ""
	}
	sccContextLogger().Debugf("Found secret %s/%s", reference.Namespace, reference.Name)

	regCode, ok := regSecret.Data[util.RegCodeSecretKey]
	if !ok {
		sccContextLogger().Warnf("registration secret `%v` does not contain expected data `%s`", reference, util.RegCodeSecretKey)
		return ""
	}

	return string(regCode)
}

func CreateSccOfflineRegistrationRequestSecret(offlineBlob []byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.RancherSCCOfflineRequestSecretName,
			Namespace: "cattle-system",
		},
		Data: map[string][]byte{
			util.RegCertSecretKey: offlineBlob,
		},
	}
}

func StoreSccOfflineRegistration(secrets controllerv1.SecretController, offlineBlob []byte) (*corev1.Secret, error) {
	newSecret := CreateSccOfflineRegistrationRequestSecret(offlineBlob)
	existingSecret, err := secrets.Get(newSecret.Namespace, newSecret.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, fmt.Errorf("error checking if offline request secret exists: %v", err)
	}
	var createdOrUpdated *corev1.Secret
	if apierrors.IsNotFound(err) {
		created, err := secrets.Create(newSecret)
		if err != nil {
			return nil, err
		}
		createdOrUpdated = created
	} else {
		existingSecret.Data = newSecret.Data
		updated, err := secrets.Update(existingSecret)
		if err != nil {
			return nil, err
		}
		createdOrUpdated = updated
	}

	// TODO: update Request status to point to creds secret
	return createdOrUpdated, nil
}
