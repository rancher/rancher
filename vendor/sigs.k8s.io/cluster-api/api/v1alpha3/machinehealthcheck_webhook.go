/*
Copyright 2020 The Kubernetes Authors.

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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	// Default time allowed for a node to start up. Can be made longer as part of
	// spec if required for particular provider.
	// 10 minutes should allow the instance to start and the node to join the
	// cluster on most providers.
	defaultNodeStartupTimeout = metav1.Duration{Duration: 10 * time.Minute}
)

func (m *MachineHealthCheck) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-cluster-x-k8s-io-v1alpha3-machinehealthcheck,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinehealthchecks,versions=v1alpha3,name=validation.machinehealthcheck.cluster.x-k8s.io
// +kubebuilder:webhook:verbs=create;update,path=/mutate-cluster-x-k8s-io-v1alpha3-machinehealthcheck,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=cluster.x-k8s.io,resources=machinehealthchecks,versions=v1alpha3,name=default.machinehealthcheck.cluster.x-k8s.io

var _ webhook.Defaulter = &MachineHealthCheck{}
var _ webhook.Validator = &MachineHealthCheck{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (m *MachineHealthCheck) Default() {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[ClusterLabelName] = m.Spec.ClusterName

	if m.Spec.MaxUnhealthy == nil {
		defaultMaxUnhealthy := intstr.FromString("100%")
		m.Spec.MaxUnhealthy = &defaultMaxUnhealthy
	}

	if m.Spec.NodeStartupTimeout == nil {
		m.Spec.NodeStartupTimeout = &defaultNodeStartupTimeout
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (m *MachineHealthCheck) ValidateCreate() error {
	return m.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (m *MachineHealthCheck) ValidateUpdate(old runtime.Object) error {
	mhc, ok := old.(*MachineHealthCheck)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a MachineHealthCheck but got a %T", old))
	}
	return m.validate(mhc)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (m *MachineHealthCheck) ValidateDelete() error {
	return nil
}

func (m *MachineHealthCheck) validate(old *MachineHealthCheck) error {
	var allErrs field.ErrorList

	// Validate selector parses as Selector
	_, err := metav1.LabelSelectorAsSelector(&m.Spec.Selector)
	if err != nil {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "selector"), m.Spec.Selector, err.Error()),
		)
	}

	if old != nil && old.Spec.ClusterName != m.Spec.ClusterName {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "clusterName"), m.Spec.ClusterName, "field is immutable"),
		)
	}

	if m.Spec.NodeStartupTimeout != nil && m.Spec.NodeStartupTimeout.Seconds() < 30 {
		allErrs = append(
			allErrs,
			field.Invalid(field.NewPath("spec", "nodeStartupTimeout"), m.Spec.NodeStartupTimeout, "must be at least 30s"),
		)
	}

	if m.Spec.MaxUnhealthy != nil {
		if _, err := intstr.GetValueFromIntOrPercent(m.Spec.MaxUnhealthy, 0, false); err != nil {
			allErrs = append(
				allErrs,
				field.Invalid(field.NewPath("spec", "maxUnhealthy"), m.Spec.MaxUnhealthy, "must be either an int or a percentage"),
			)
		} else if m.Spec.MaxUnhealthy.Type == intstr.String {
			if len(validation.IsValidPercent(m.Spec.MaxUnhealthy.StrVal)) != 0 {
				allErrs = append(
					allErrs,
					field.Invalid(field.NewPath("spec", "maxUnhealthy"), m.Spec.MaxUnhealthy, "must be either an int or a percentage"),
				)
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("MachineHealthCheck").GroupKind(), m.Name, allErrs)
}
