package controllers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/consts"
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

func (h *handler) isRancherEntrypointSecret(secretObj *corev1.Secret) bool {
	if secretObj.Name != consts.ResourceSCCEntrypointSecretName || secretObj.Namespace != h.systemNamespace {
		return false
	}
	return true
}

func extraRegistrationParamsFromSecret(secret *corev1.Secret) (RegistrationParams, error) {
	regMode := v1.RegistrationModeOnline
	regType, ok := secret.Data[dataRegistrationType]
	if ok && len(regType) > 0 {
		regMode = v1.RegistrationMode(regType)
		if !regMode.Valid() {
			return RegistrationParams{}, fmt.Errorf("invalid registration mode %s", string(regType))
		}
	}

	regCode, ok := secret.Data[dataRegCode]
	if regMode == v1.RegistrationModeOnline && (!ok || len(regCode) == 0) {
		return RegistrationParams{}, fmt.Errorf("secret does not have data %s", dataRegCode)
	}

	hasher := md5.New()
	data := append([]byte(regMode), regCode...)
	if _, err := hasher.Write(data); err != nil {
		return RegistrationParams{}, err
	}

	id := hex.EncodeToString(hasher.Sum(nil))

	return RegistrationParams{
		hash:    id,
		regCode: string(regCode),
		regType: regMode,
		secretRef: &corev1.SecretReference{
			Name:      consts.ResourceSCCEntrypointSecretName,
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
		consts.LabelSccHash:      r.hash,
		consts.LabelSccManagedBy: consts.ManagedBySecretBroker,
	}
}

func (h *handler) registrationFromSecretEntrypoint(
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
	if !slices.Contains(reg.Finalizers, consts.FinalizerSccRegistration) {
		if reg.Finalizers == nil {
			reg.Finalizers = []string{}
		}
		reg.Finalizers = append(reg.Finalizers, consts.FinalizerSccRegistration)
	}
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
