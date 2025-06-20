package controllers

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/util"
	"maps"
	"slices"

	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HashType int

const (
	NameHash HashType = iota
	ContentHash
)

func (ht *HashType) String() string {
	if *ht == NameHash {
		return "name"
	}

	return "content"
}

type hashCleanupRequest struct {
	hash     string
	hashType any
}

const (
	dataKeyRegistrationType = "registrationType"
)

func (h *handler) isRancherEntrypointSecret(secretObj *corev1.Secret) bool {
	if secretObj.Name != consts.ResourceSCCEntrypointSecretName || secretObj.Namespace != h.systemNamespace {
		return false
	}
	return true
}

func (h *handler) prepareSecretSalt(secret *corev1.Secret) (*corev1.Secret, error) {
	preparedSecret := secret.DeepCopy()
	generatedSalt := util.NewSaltGen(nil, nil).GenerateSalt()

	existingLabels := make(map[string]string)
	if objLabels := secret.GetLabels(); objLabels != nil {
		existingLabels = objLabels
	}
	existingLabels[consts.LabelObjectSalt] = generatedSalt
	preparedSecret.SetLabels(existingLabels)

	_, updateErr := h.updateSecret(secret, preparedSecret)
	if updateErr != nil {
		h.log.Error("error applying metadata updates to default SCC registration secret; cannot initialize secret salt value")
		return nil, updateErr
	}

	return secret, nil
}

func extractRegistrationParamsFromSecret(secret *corev1.Secret) (RegistrationParams, error) {
	incomingSalt := []byte(secret.GetLabels()[consts.LabelObjectSalt])

	regMode := v1.RegistrationModeOnline
	regType, ok := secret.Data[dataKeyRegistrationType]
	if !ok || len(regType) == 0 {
		// h.log.Warnf("secret does not have the `%s` field, defaulting to %s", dataKeyRegistrationType, regMode)
	} else {
		regMode = v1.RegistrationMode(regType)
		if !regMode.Valid() {
			return RegistrationParams{}, fmt.Errorf("invalid registration mode %s", string(regMode))
		}
	}

	regCode, ok := secret.Data[consts.SecretKeyRegistrationCode]
	if !ok || len(regCode) == 0 {
		if regMode == v1.RegistrationModeOnline {
			return RegistrationParams{}, fmt.Errorf("secret does not have data %s; this is required in online mode", consts.SecretKeyRegistrationCode)
		}
	}

	offlineRegCertData, certOk := secret.Data[consts.SecretKeyOfflineRegCert]
	hasOfflineCert := certOk && len(offlineRegCertData) > 0

	/*
		The Registration URL precedence is:
			- The value from the entrypoint secret (if set),
			- The Global Env value (if set),
			- Staging SCC (Only, if Dev mode)
			- Nothing (Prod SCC; default) [also used for offline mode]
	*/
	var regUrlBytes []byte
	regUrlString := ""
	if regMode == v1.RegistrationModeOnline {
		if consts.IsDevMode() {
			regUrlBytes = []byte(consts.RegistrationUrl)
			regUrlString = string(consts.StagingSCCUrl)
		}

		regUrlBytes, ok = secret.Data[consts.RegistrationUrl]
		if ok && len(regUrlBytes) != 0 {
			regUrlString = string(regUrlBytes)
		} else if util.HasGlobalPrimeRegistrationUrl() {
			globalRegistrationUrl := util.GetGlobalPrimeRegistrationUrl()
			regUrlBytes = []byte(globalRegistrationUrl)
			regUrlString = globalRegistrationUrl
		}
	}

	// TODO: do we care to validate this; online shouldn't have this at all, offline has it eventually
	// So it cannot be required for offline mode like RegCode is above, we could error online mode with it?

	hasher := md5.New()
	nameData := append(incomingSalt, regType...)
	nameData = append(nameData, regCode...)
	nameData = append(nameData, regUrlBytes...)
	data := append(nameData, offlineRegCertData...)

	// Generate a has for the name data
	if _, err := hasher.Write(nameData); err != nil {
		return RegistrationParams{}, fmt.Errorf("failed to hash name data: %v", err)
	}
	nameId := hex.EncodeToString(hasher.Sum(nil))

	// Generate hash for the content data
	if _, err := hasher.Write(data); err != nil {
		return RegistrationParams{}, fmt.Errorf("failed to hash data: %v", err)
	}
	contentsId := hex.EncodeToString(hasher.Sum(nil))

	return RegistrationParams{
		regType:            regMode,
		nameId:             nameId,
		contentHash:        contentsId,
		regCode:            string(regCode),
		hasOfflineCertData: hasOfflineCert,
		offlineCertData:    &offlineRegCertData,
		secretRef: &corev1.SecretReference{
			Name:      consts.ResourceSCCEntrypointSecretName,
			Namespace: secret.Namespace,
		},
		regUrl: regUrlString,
	}, nil
}

type RegistrationParams struct {
	regType            v1.RegistrationMode
	nameId             string
	contentHash        string
	regCode            string
	regUrl             string
	hasOfflineCertData bool
	secretRef          *corev1.SecretReference
	offlineCertData    *[]byte
}

func (r RegistrationParams) Labels() map[string]string {
	return map[string]string{
		consts.LabelNameSuffix:   r.nameId,
		consts.LabelSccHash:      r.contentHash,
		consts.LabelSccManagedBy: consts.ManagedBySecretBroker,
	}
}

func (h *handler) registrationFromSecretEntrypoint(
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
	genName := fmt.Sprintf("scc-registration-%s", params.nameId)
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
	if reg.Labels == nil {
		reg.Labels = map[string]string{}
	}
	maps.Copy(reg.Labels, params.Labels())

	reg.Spec = paramsToRegSpec(params)
	if !slices.Contains(reg.Finalizers, consts.FinalizerSccRegistration) {
		if reg.Finalizers == nil {
			reg.Finalizers = []string{}
		}
		reg.Finalizers = append(reg.Finalizers, consts.FinalizerSccRegistration)
	}
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

	// check if params has regUrl and use, otherwise check if devmode and when true use staging Scc url
	if params.regUrl != "" {
		regSpec.RegistrationRequest.RegistrationUrl = &params.regUrl
	}

	return regSpec
}

func (h *handler) offlineCertFromSecretEntrypoint(params RegistrationParams) (*corev1.Secret, error) {
	secretName := consts.OfflineCertificateSecretName(params.nameId)

	offlineCertSecret, err := h.secretCache.Get(h.systemNamespace, secretName)
	if err != nil && apierrors.IsNotFound(err) {
		offlineCertSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: h.systemNamespace,
				Name:      secretName,
			},
			Data: map[string][]byte{
				consts.SecretKeyOfflineRegCert: *params.offlineCertData,
			},
		}
	}

	if offlineCertSecret.Labels == nil {
		offlineCertSecret.Labels = map[string]string{}
	}
	maps.Copy(offlineCertSecret.Labels, params.Labels())

	if !slices.Contains(offlineCertSecret.Finalizers, consts.FinalizerSccRegistration) {
		if offlineCertSecret.Finalizers == nil {
			offlineCertSecret.Finalizers = []string{}
		}
		offlineCertSecret.Finalizers = append(offlineCertSecret.Finalizers, consts.FinalizerSccRegistration)
	}

	return offlineCertSecret, nil
}
