package common

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/types"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"slices"
)

// GetRegistrationProcessors returns all shared processors
func GetRegistrationProcessors() []types.RegistrationProcessor {
	return []types.RegistrationProcessor{}
}

// GetRegistrationStatusProcessors returns all shared processors
func GetRegistrationStatusProcessors() []types.RegistrationStatusProcessor {
	return []types.RegistrationStatusProcessor{
		PrepareSuccessfulActivation,
	}
}

func PrepareSuccessfulActivation(regIn *v1.Registration) *v1.Registration {
	now := metav1.Now()
	v1.RegistrationConditionActivated.True(regIn)
	v1.ResourceConditionProgressing.False(regIn)
	v1.ResourceConditionReady.True(regIn)
	v1.ResourceConditionDone.True(regIn)
	regIn.Status.ActivationStatus.LastValidatedTS = &now
	regIn.Status.ActivationStatus.Activated = true

	return regIn
}

func GetSecretProcessors() []types.Processor[*corev1.Secret] {
	return []types.Processor[*corev1.Secret]{
		SecretAddCredentialsFinalizer,
		SecretRemoveCredentialsFinalizer,
		SecretAddRegCodeFinalizer,
		SecretRemoveRegCodeFinalizer,
		SecretAddOfflineFinalizer,
		SecretRemoveOfflineFinalizer,
	}
}

func secretAddFinalizer[T generic.RuntimeMetaObject](objIn T, finalizer string) T {
	finalizers := objIn.GetFinalizers()
	if finalizers == nil {
		objIn.SetFinalizers([]string{finalizer})
		return objIn
	}

	if !slices.Contains(finalizers, finalizer) {
		objIn.SetFinalizers(append(objIn.GetFinalizers(), finalizer))
	}

	return objIn
}

func secretRemoveFinalizer[T generic.RuntimeMetaObject](objIn T, finalizer string) T {
	finalizers := objIn.GetFinalizers()
	if finalizers == nil {
		return objIn
	}

	index := slices.Index(finalizers, finalizer)
	if index == -1 {
		return objIn
	}

	finalizers = slices.Delete(finalizers, index, index+1)
	objIn.SetFinalizers(finalizers)
	return objIn
}

func SecretAddCredentialsFinalizer(secret *corev1.Secret) (*corev1.Secret, error) {
	return secretAddFinalizer[*corev1.Secret](secret, consts.FinalizerSccCredentials), nil
}

func SecretRemoveCredentialsFinalizer(secret *corev1.Secret) (*corev1.Secret, error) {
	return secretRemoveFinalizer[*corev1.Secret](secret, consts.FinalizerSccCredentials), nil
}

func SecretAddRegCodeFinalizer(secret *corev1.Secret) (*corev1.Secret, error) {
	return secretAddFinalizer[*corev1.Secret](secret, consts.FinalizerSccRegistrationCode), nil
}

func SecretRemoveRegCodeFinalizer(secret *corev1.Secret) (*corev1.Secret, error) {
	return secretRemoveFinalizer[*corev1.Secret](secret, consts.FinalizerSccRegistrationCode), nil
}

func SecretAddOfflineFinalizer(secret *corev1.Secret) (*corev1.Secret, error) {
	return secretAddFinalizer[*corev1.Secret](secret, consts.FinalizerSccOfflineSecret), nil
}

func SecretRemoveOfflineFinalizer(secret *corev1.Secret) (*corev1.Secret, error) {
	return secretRemoveFinalizer[*corev1.Secret](secret, consts.FinalizerSccOfflineSecret), nil
}
