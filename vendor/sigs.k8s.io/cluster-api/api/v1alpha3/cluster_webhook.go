/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha3

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1alpha3-cluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=clusters,versions=v1alpha3,name=validation.cluster.cluster.x-k8s.io
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1alpha3-cluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=clusters,versions=v1alpha3,name=default.cluster.cluster.x-k8s.io

var _ webhook.Defaulter = &Cluster{}
var _ webhook.Validator = &Cluster{}

func (c *Cluster) Default() {
	if c.Spec.InfrastructureRef != nil && len(c.Spec.InfrastructureRef.Namespace) == 0 {
		c.Spec.InfrastructureRef.Namespace = c.Namespace
	}

	if c.Spec.ControlPlaneRef != nil && len(c.Spec.ControlPlaneRef.Namespace) == 0 {
		c.Spec.ControlPlaneRef.Namespace = c.Namespace
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Cluster) ValidateCreate() error {
	return c.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Cluster) ValidateUpdate(old runtime.Object) error {
	return c.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Cluster) ValidateDelete() error {
	return nil
}

func (c *Cluster) validate() error {
	var allErrs field.ErrorList
	if c.Spec.InfrastructureRef != nil && c.Spec.InfrastructureRef.Namespace != c.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "infrastructureRef", "namespace"),
				c.Spec.InfrastructureRef.Namespace,
				"must match metadata.namespace",
			),
		)

	}

	if c.Spec.ControlPlaneRef != nil && c.Spec.ControlPlaneRef.Namespace != c.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "controlPlaneRef", "namespace"),
				c.Spec.ControlPlaneRef.Namespace,
				"must match metadata.namespace",
			),
		)

	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Cluster").GroupKind(), c.Name, allErrs)
}
