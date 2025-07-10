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

// prepareSecretSalt applies an instance salt onto an entrypoint secret used to create randomness in hashes
func (h *handler) prepareSecretSalt(secret *corev1.Secret) (*corev1.Secret, error) {
	preparedSecret := secret.DeepCopy()
	generatedSalt := util.NewSaltGen(nil, nil).GenerateSalt()

	existingLabels := make(map[string]string)
	if objLabels := secret.GetLabels(); objLabels != nil {
		existingLabels = objLabels
	}
	existingLabels[consts.LabelObjectSalt] = generatedSalt
	preparedSecret.SetLabels(existingLabels)

	_, updateErr := h.patchUpdateSecret(secret, preparedSecret)
	if updateErr != nil {
		h.log.Error("error applying metadata updates to default SCC registration secret; cannot initialize secret salt value")
		return nil, updateErr
	}

	return secret, nil
}

func getCurrentRegURL(secret *corev1.Secret) (regURL []byte) {
	regUrlBytes, ok := secret.Data[consts.RegistrationUrl]
	if ok {
		return regUrlBytes
	}
	if util.HasGlobalPrimeRegistrationUrl() {
		globalRegistrationUrl := util.GetGlobalPrimeRegistrationUrl()
		return []byte(globalRegistrationUrl)
	}
	if consts.IsDevMode() {
		return []byte(consts.StagingSCCUrl)
	}
	return []byte{}
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

	var regUrlBytes []byte
	regUrlString := ""
	if regMode == v1.RegistrationModeOnline {
		regUrlBytes = getCurrentRegURL(secret)
		regUrlString = string(regUrlBytes)
	}

	hasher := md5.New()
	nameData := append(incomingSalt, regType...)
	nameData = append(nameData, regCode...)
	nameData = append(nameData, regUrlBytes...)
	data := append(nameData, offlineRegCertData...)

	// Generate a hash for the name data
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
		regType:     regMode,
		nameId:      nameId,
		contentHash: contentsId,
		regCode:     regCode,
		regCodeSecretRef: &corev1.SecretReference{
			Name:      consts.RegistrationCodeSecretName(nameId),
			Namespace: secret.Namespace,
		},
		hasOfflineCertData: hasOfflineCert,
		offlineCertData:    &offlineRegCertData,
		offlineCertSecretRef: &corev1.SecretReference{
			Name:      consts.OfflineCertificateSecretName(nameId),
			Namespace: secret.Namespace,
		},
		regUrl: regUrlString,
	}, nil
}

type RegistrationParams struct {
	regType              v1.RegistrationMode
	nameId               string
	contentHash          string
	regCode              []byte
	regCodeSecretRef     *corev1.SecretReference
	regUrl               string
	hasOfflineCertData   bool
	offlineCertData      *[]byte
	offlineCertSecretRef *corev1.SecretReference
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
			RegistrationCodeSecretRef: params.regCodeSecretRef,
		}
	} else if params.regType == v1.RegistrationModeOffline && params.hasOfflineCertData {
		regSpec.OfflineRegistrationCertificateSecretRef = params.offlineCertSecretRef
	}

	// check if params has regUrl and use, otherwise check if devmode and when true use staging Scc url
	if params.regUrl != "" {
		regSpec.RegistrationRequest.RegistrationAPIUrl = &params.regUrl
	}

	return regSpec
}

func (h *handler) regCodeFromSecretEntrypoint(params RegistrationParams) (*corev1.Secret, error) {
	secretName := params.regCodeSecretRef.Name

	regcodeSecret, err := h.secretCache.Get(h.systemNamespace, secretName)
	if err != nil && apierrors.IsNotFound(err) {
		regcodeSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: h.systemNamespace,
				Name:      secretName,
			},
			Data: map[string][]byte{
				consts.SecretKeyRegistrationCode: params.regCode,
			},
		}
	}

	if regcodeSecret.Labels == nil {
		regcodeSecret.Labels = map[string]string{}
	}
	defaultLabels := params.Labels()
	defaultLabels[consts.LabelSccSecretRole] = string(consts.RegistrationCode)
	maps.Copy(regcodeSecret.Labels, defaultLabels)

	if !slices.Contains(regcodeSecret.Finalizers, consts.FinalizerSccRegistrationCode) {
		if regcodeSecret.Finalizers == nil {
			regcodeSecret.Finalizers = []string{}
		}
		regcodeSecret.Finalizers = append(regcodeSecret.Finalizers, consts.FinalizerSccRegistrationCode)
	}

	return regcodeSecret, nil
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
	defaultLabels := params.Labels()
	defaultLabels[consts.LabelSccSecretRole] = string(consts.OfflineCertificate)
	maps.Copy(offlineCertSecret.Labels, defaultLabels)

	if !slices.Contains(offlineCertSecret.Finalizers, consts.FinalizerSccRegistration) {
		if offlineCertSecret.Finalizers == nil {
			offlineCertSecret.Finalizers = []string{}
		}
		offlineCertSecret.Finalizers = append(offlineCertSecret.Finalizers, consts.FinalizerSccRegistration)
	}

	return offlineCertSecret, nil
}
