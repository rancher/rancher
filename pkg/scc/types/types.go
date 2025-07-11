package types

import (
	v1 "github.com/rancher/rancher/pkg/apis/scc.cattle.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Decider makes a decision based on the state of a generic Kubernetes resource; must have no side-effects.
// The type parameter `T` is constrained to `client.Object`, meaning `T` must be any type
// that implements the `client.Object` interface (which all K8s API types do).
type Decider[T client.Object] func(T) bool

// Processor will update the state of a generic Kubernetes resource with potential for error.
// The type parameter `T` is constrained to `client.Object`.
type Processor[T client.Object] func(T) (T, error)

// Mutator will update the state of a generic Kubernetes resource in-memory.
// The type parameter `T` is constrained to `client.Object`.
type Mutator[T client.Object] func(T) T

// RegistrationDecider makes a decision based on registration state; must have no side-effects
type RegistrationDecider Decider[*v1.Registration]

// RegistrationProcessor will update the state of the registration object with potential for error
type RegistrationProcessor Processor[*v1.Registration]

// RegistrationStatusProcessor handles updates to registration status
type RegistrationStatusProcessor Mutator[*v1.Registration]

// RegistrationFailureReconciler helps to reconcile Registration state after errors
type RegistrationFailureReconciler func(*v1.Registration, error) *v1.Registration

type RegistrationReconcileRetry func() error

type HandlerReconcileErrorProcessor func(*v1.Registration, error, Phase) *v1.Registration
