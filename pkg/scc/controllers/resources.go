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
	dataCertificate      = "certificate"
)

func (h *handler) isRancherEntrypointSecret(secretObj *corev1.Secret) bool {
	if secretObj.Name != consts.ResourceSCCEntrypointSecretName || secretObj.Namespace != h.systemNamespace {
		return false
	}
	return true
}

func extraRegistrationParamsFromSecret(secret *corev1.Secret) (RegistrationParams, error) {
	regType, ok := secret.Data[dataRegistrationType]
	if !ok || len(regType) == 0 {
		return RegistrationParams{}, fmt.Errorf("secret does not have label %s", dataRegistrationType)
	}
	regMode := v1.RegistrationMode(regType)
	if !regMode.Valid() {
		return RegistrationParams{}, fmt.Errorf("invalid registration mode %s", string(regMode))
	}

	regCode, ok := secret.Data[dataRegCode]
	if regMode == v1.RegistrationModeOnline && (!ok || len(regCode) == 0) {
		return RegistrationParams{}, fmt.Errorf("secret does not have data %s; this is required in online mode", dataRegCode)
	}

	offlineRegCert, certOk := secret.Data[dataCertificate]
	// TODO: do we care to validate this; online shouldn't have this at all, offline has it eventually

	hasher := md5.New()
	data := append(regCode, regType...)
	// TODO: we both want the RegCert included and do not want it included; it should update Registration when it changes, but not change the name ideally
	data = append(data, offlineRegCert...)
	if _, err := hasher.Write(data); err != nil {
		return RegistrationParams{}, err
	}

	id := hex.EncodeToString(hasher.Sum(nil))

	return RegistrationParams{
		hash:               id,
		regCode:            string(regCode),
		hasOfflineCertData: certOk && len(offlineRegCert) > 0,
		regType:            v1.RegistrationMode(regType),
		secretRef: &corev1.SecretReference{
			Name:      consts.ResourceSCCEntrypointSecretName,
			Namespace: secret.Namespace,
		},
	}, nil
}

type RegistrationParams struct {
	hash               string
	regCode            string
	hasOfflineCertData bool
	regType            v1.RegistrationMode
	secretRef          *corev1.SecretReference
}

func (r RegistrationParams) Labels() map[string]string {
	return map[string]string{
		consts.LabelSccHash:      r.hash,
		consts.LabelSccManagedBy: consts.ManagedBySecretBroker,
	}
}

func (h *handler) registrationFromSecretEntrypoint(
	ownerRef metav1.OwnerReference,
	params RegistrationParams,
) (*v1.Registration, error) {
	if !params.regType.Valid() {
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
	reg.Spec = paramsToRegSpec(params)
	if !slices.Contains(reg.Finalizers, consts.FinalizerSccRegistration) {
		if reg.Finalizers == nil {
			reg.Finalizers = []string{}
		}
		reg.Finalizers = append(reg.Finalizers, consts.FinalizerSccRegistration)
	}
	reg.OwnerReferences = []metav1.OwnerReference{ownerRef}
	return reg, nil
}

func paramsToRegSpec(params RegistrationParams) v1.RegistrationSpec {
	regSpec := v1.RegistrationSpec{
		Mode: params.regType,
	}

	if params.regType == v1.RegistrationModeOnline {
		regSpec.RegistrationRequest = &v1.RegistrationRequest{
			RegistrationCodeSecretRef: params.secretRef,
		}
	} else if params.regType == v1.RegistrationModeOffline && params.hasOfflineCertData {
		regSpec.OfflineRegistrationCertificateSecretRef = params.secretRef
	}

	return regSpec
}
