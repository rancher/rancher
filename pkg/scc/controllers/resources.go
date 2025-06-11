package controllers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
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
	ResourceSCCEntrypointSecretName = "scc-registration"
)

func (h *handler) isRancherEntrypointSecret(secretObj *corev1.Secret) bool {
	if secretObj.Name != ResourceSCCEntrypointSecretName || secretObj.Namespace != h.systemNamespace {
		return false
	}
	return true
}

func (h *handler) isRancherSccSecret(secretObj *corev1.Secret) bool {
	if !strings.HasPrefix(secretObj.Name, consts.SCCSystemCredentialsSecretNamePrefix) || secretObj.Namespace != h.systemNamespace {
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
		LabelSccManagedBy: "",
	}
}

func registrationFromSecretEntrypoint(
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

	registration := &v1.Registration{
		ObjectMeta: metav1.ObjectMeta{
			// FIXME: lets figure how to generate better unique names
			Name:   fmt.Sprintf("scc-registration-%s", params.hash),
			Labels: params.Labels(),
		},
		Spec: v1.RegistrationSpec{
			Mode: params.regType,
			RegistrationRequest: &v1.RegistrationRequest{
				RegistrationCodeSecretRef: params.secretRef,
				// TODO: set fields for non-SCC based RMT regs too
			},
		},
	}

	return registration, nil
}
