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
	"fmt"
	"strings"

	"github.com/blang/semver"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (m *Machine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1alpha3-machine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machines,versions=v1alpha3,name=validation.machine.cluster.x-k8s.io
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1alpha3-machine,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machines,versions=v1alpha3,name=default.machine.cluster.x-k8s.io

var _ webhook.Validator = &Machine{}
var _ webhook.Defaulter = &Machine{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (m *Machine) Default() {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[ClusterLabelName] = m.Spec.ClusterName

	if m.Spec.Bootstrap.ConfigRef != nil && len(m.Spec.Bootstrap.ConfigRef.Namespace) == 0 {
		m.Spec.Bootstrap.ConfigRef.Namespace = m.Namespace
	}

	if len(m.Spec.InfrastructureRef.Namespace) == 0 {
		m.Spec.InfrastructureRef.Namespace = m.Namespace
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (m *Machine) ValidateCreate() error {
	return m.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (m *Machine) ValidateUpdate(old runtime.Object) error {
	oldM, ok := old.(*Machine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Machine but got a %T", old))
	}
	return m.validate(oldM)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (m *Machine) ValidateDelete() error {
	return nil
}

func (m *Machine) validate(old *Machine) error {
	var allErrs field.ErrorList
	if m.Spec.Bootstrap.ConfigRef == nil && m.Spec.Bootstrap.DataSecretName == nil {
		allErrs = append(
			allErrs,
			field.Required(
				field.NewPath("spec", "bootstrap", "data"),
				"expected either spec.bootstrap.dataSecretName or spec.bootstrap.configRef to be populated",
			),
		)
	}

	if m.Spec.Bootstrap.ConfigRef != nil && m.Spec.Bootstrap.ConfigRef.Namespace != m.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "bootstrap", "configRef", "namespace"),
				m.Spec.Bootstrap.ConfigRef.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	if m.Spec.InfrastructureRef.Namespace != m.Namespace {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "infrastructureRef", "namespace"),
				m.Spec.InfrastructureRef.Namespace,
				"must match metadata.namespace",
			),
		)
	}

	if old != nil && old.Spec.ClusterName != m.Spec.ClusterName {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "clusterName"), m.Spec.ClusterName, "field is immutable"),
		)
	}

	if m.Spec.Version != nil {
		if _, err := semver.Parse(strings.TrimPrefix(strings.TrimSpace(*m.Spec.Version), "v")); err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "version"), *m.Spec.Version, "must be a valid semantic version"))
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Machine").GroupKind(), m.Name, allErrs)
}
