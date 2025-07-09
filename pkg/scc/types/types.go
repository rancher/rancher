package types

import v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"

// RegistrationDecider makes a decision based on registration state; must have no side-effects
type RegistrationDecider func(*v1.Registration) bool

// RegistrationProcessor will update the state of the registration object with potential for error
type RegistrationProcessor func(*v1.Registration) (*v1.Registration, error)

// RegistrationStatusProcessor handles updates to registration status
type RegistrationStatusProcessor func(*v1.Registration) *v1.Registration

// RegistrationFailureReconciler helps to reconcile Registration state after errors
type RegistrationFailureReconciler func(*v1.Registration, error) *v1.Registration

type RegistrationReconcileRetry func() error

type HandlerReconcileErrorProcessor func(*v1.Registration, error, Phase) *v1.Registration
