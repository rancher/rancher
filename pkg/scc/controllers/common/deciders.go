package common

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"github.com/rancher/rancher/pkg/scc/consts"
	"github.com/rancher/rancher/pkg/scc/types"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	"slices"
)

// GetRegistrationDeciders returns all shared deciders
func GetRegistrationDeciders() []types.RegistrationDecider {
	return []types.RegistrationDecider{
		RegistrationIsFailed,
		RegistrationNeedsSyncNow,
		RegistrationHasNotStarted,
		RegistrationNeedsActivation,
		RegistrationHasManagedFinalizer,
	}
}

func RegistrationIsFailed(regIn *v1.Registration) bool {
	return regIn.HasCondition(v1.ResourceConditionFailure) && v1.ResourceConditionFailure.IsTrue(regIn)
}

func RegistrationNeedsSyncNow(regIn *v1.Registration) bool {
	return regIn.Spec.SyncNow != nil && *regIn.Spec.SyncNow
}

func RegistrationHasNotStarted(regIn *v1.Registration) bool {
	return regIn.Status.RegistrationProcessedTS.IsZero()
}

func RegistrationNeedsActivation(regIn *v1.Registration) bool {
	return regIn.Status.RegistrationProcessedTS.IsZero() ||
		!regIn.Status.ActivationStatus.Activated
}

func RegistrationHasManagedFinalizer(objIn *v1.Registration) bool {
	return hasFinalizer(objIn, consts.FinalizerSccRegistration)
}

func GetSecretDeciders() []types.Decider[*corev1.Secret] {
	return []types.Decider[*corev1.Secret]{
		SecretHasOfflineFinalizer,
		SecretHasCredentialsFinalizer,
		SecretHasRegCodeFinalizer,
	}
}

func hasFinalizer[T generic.RuntimeMetaObject](objIn T, finalizer string) bool {
	finalizers := objIn.GetFinalizers()
	if finalizers == nil {
		return false
	}

	return slices.Contains(finalizers, finalizer)
}

func SecretHasOfflineFinalizer(objIn *corev1.Secret) bool {
	return hasFinalizer(objIn, consts.FinalizerSccOfflineSecret)
}

func SecretHasCredentialsFinalizer(objIn *corev1.Secret) bool {
	return hasFinalizer(objIn, consts.FinalizerSccCredentials)
}

func SecretHasRegCodeFinalizer(objIn *corev1.Secret) bool {
	return hasFinalizer(objIn, consts.FinalizerSccRegistrationCode)
}
