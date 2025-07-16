package common

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/types"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	"slices"
)

func GetRegistrationMutators() []types.Mutator[*v1.Registration] {
	return []types.Mutator[*v1.Registration]{
		RegistrationAddManagedFinalizer,
	}
}

func RegistrationAddManagedFinalizer(registration *v1.Registration) *v1.Registration {
	return runtimeAddFinalizer(registration, consts.FinalizerSccRegistration)
}

func GetSecretMutators() []types.Mutator[*corev1.Secret] {
	return []types.Mutator[*corev1.Secret]{
		SecretAddCredentialsFinalizer,
		SecretRemoveCredentialsFinalizer,
		SecretAddRegCodeFinalizer,
		SecretRemoveRegCodeFinalizer,
		SecretAddOfflineFinalizer,
		SecretRemoveOfflineFinalizer,
	}
}

func runtimeAddFinalizer[T generic.RuntimeMetaObject](objIn T, finalizer string) T {
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

func runtimeRemoveFinalizer[T generic.RuntimeMetaObject](objIn T, finalizer string) T {
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

func SecretAddCredentialsFinalizer(secret *corev1.Secret) *corev1.Secret {
	return runtimeAddFinalizer[*corev1.Secret](secret, consts.FinalizerSccCredentials)
}

func SecretRemoveCredentialsFinalizer(secret *corev1.Secret) *corev1.Secret {
	return runtimeRemoveFinalizer[*corev1.Secret](secret, consts.FinalizerSccCredentials)
}

func SecretAddRegCodeFinalizer(secret *corev1.Secret) *corev1.Secret {
	return runtimeAddFinalizer[*corev1.Secret](secret, consts.FinalizerSccRegistrationCode)
}

func SecretRemoveRegCodeFinalizer(secret *corev1.Secret) *corev1.Secret {
	return runtimeRemoveFinalizer[*corev1.Secret](secret, consts.FinalizerSccRegistrationCode)
}

func SecretAddOfflineFinalizer(secret *corev1.Secret) *corev1.Secret {
	return runtimeAddFinalizer[*corev1.Secret](secret, consts.FinalizerSccOfflineSecret)
}

func SecretRemoveOfflineFinalizer(secret *corev1.Secret) *corev1.Secret {
	return runtimeRemoveFinalizer[*corev1.Secret](secret, consts.FinalizerSccOfflineSecret)
}
