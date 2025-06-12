package controllers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"slices"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	dataRegCode          = "regCode"
	dataRegistrationType = "registrationType"
)
const (
	LabelSccLastProcessed = "scc.cattle.io/last-processsed"
	LabelSccHash          = "scc.cattle.io/scc-hash"
	LabelSccManagedBy     = "scc.cattle.io/managed-by"
)

const (
	ManagedBySecretBroker = "secret-broker"
)

const (
	FinalizerSccCredentials  = "scc.cattle.io/managed-credentials"
	FinalizerSccRegistration = "scc.cattle.io/managed-registration"
)

const (
	ResourceSCCEntrypointSecretName = "scc-registration"
)

func (h *handler) isRancherEntrypointSecret(secretObj *corev1.Secret) bool {
	if secretObj.Name != ResourceSCCEntrypointSecretName || secretObj.Namespace != h.systemNamespace {
		return false
	}
	return true
}

func extraRegistrationParamsFromSecret(secret *corev1.Secret) (RegistrationParams, error) {
	regCode, ok := secret.Data[dataRegCode]
	if !ok || len(regCode) == 0 {
		return RegistrationParams{}, fmt.Errorf("secret does not have data %s", dataRegCode)
	}

	regType, ok := secret.Data[dataRegistrationType]
	if !ok || len(regType) == 0 {
		return RegistrationParams{}, fmt.Errorf("secret does not have label %s", dataRegistrationType)
	}
	hasher := md5.New()
	data := append(regCode, regType...)
	if _, err := hasher.Write(data); err != nil {
		return RegistrationParams{}, err
	}

	id := hex.EncodeToString(hasher.Sum(nil))

	return RegistrationParams{
		hash:    id,
		regCode: string(regCode),
		regType: v1.RegistrationMode(regType),
		secretRef: &corev1.SecretReference{
			Name:      ResourceSCCEntrypointSecretName,
			Namespace: secret.Namespace,
		},
	}, nil
}

type RegistrationParams struct {
	hash      string
	regCode   string
	regType   v1.RegistrationMode
	secretRef *corev1.SecretReference
}

func (r RegistrationParams) Labels() map[string]string {
	return map[string]string{
		LabelSccHash:      r.hash,
		LabelSccManagedBy: ManagedBySecretBroker,
	}
}

func (h *handler) registrationFromSecretEntrypoint(
	ownerRef metav1.OwnerReference,
	params RegistrationParams,
) (*v1.Registration, error) {
	if params.regType != v1.RegistrationModeOnline && params.regType != v1.RegistrationModeOffline {
		return nil, fmt.Errorf(
			"invalid registration type %s, must be one of %s or %s",
			params.regType,
			v1.RegistrationModeOnline,
			v1.RegistrationModeOffline,
		)
	}

	// FIXME: lets figure how to generate better unique names
	genName := fmt.Sprintf("scc-registration-%s", params.hash)
	var reg *v1.Registration
	var err error

	reg, err = h.registrationCache.Get(genName)
	if err != nil && apierrors.IsNotFound(err) {
		reg = &v1.Registration{
			ObjectMeta: metav1.ObjectMeta{
				Name: genName,
			},
		}
	}

	reg.Labels = params.Labels()
	reg.Spec = paramsToReg(params)
	if !slices.Contains(reg.Finalizers, FinalizerSccRegistration) {
		if reg.Finalizers == nil {
			reg.Finalizers = []string{}
		}
		reg.Finalizers = append(reg.Finalizers, FinalizerSccRegistration)
	}
	reg.OwnerReferences = []metav1.OwnerReference{ownerRef}
	return reg, nil
}

func paramsToReg(params RegistrationParams) v1.RegistrationSpec {
	return v1.RegistrationSpec{
		Mode: params.regType,
		RegistrationRequest: &v1.RegistrationRequest{
			RegistrationCodeSecretRef: params.secretRef,
		},
	}
}
