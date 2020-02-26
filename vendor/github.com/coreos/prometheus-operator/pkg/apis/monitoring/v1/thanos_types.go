// Copyright 2020 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ThanosRulerKind    = "ThanosRuler"
	ThanosRulerName    = "thanosrulers"
	ThanosRulerKindKey = "thanosrulers"
)

// ThanosRuler defines a ThanosRuler deployment.
// +genclient
// +k8s:openapi-gen=true
type ThanosRuler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the ThanosRuler cluster. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec ThanosRulerSpec `json:"spec"`
	// Most recent observed status of the ThanosRuler cluster. Read-only. Not
	// included when requesting from the apiserver, only from the ThanosRuler
	// Operator API itself. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *ThanosRulerStatus `json:"status,omitempty"`
}

// ThanosRulerList is a list of ThanosRulers.
// +k8s:openapi-gen=true
type ThanosRulerList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	// List of Prometheuses
	Items []*ThanosRuler `json:"items"`
}

// ThanosRulerSpec is a specification of the desired behavior of the ThanosRuler. More info:
// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
// +k8s:openapi-gen=true
type ThanosRulerSpec struct {
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#metadata
	// Metadata Labels and Annotations gets propagated to the prometheus pods.
	PodMetadata *metav1.ObjectMeta `json:"podMetadata,omitempty"`
	// Thanos container image URL.
	Image string `json:"image,omitempty"`
	// An optional list of references to secrets in the same namespace
	// to use for pulling thanos images from registries
	// see http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// When a ThanosRuler deployment is paused, no actions except for deletion
	// will be performed on the underlying objects.
	Paused bool `json:"paused,omitempty"`
	// Number of thanos ruler instances to deploy.
	Replicas *int32 `json:"replicas,omitempty"`
	// Resources defines the resource requirements for the Thanos sidecar.
	// If not provided, no requests/limits will be set
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// Storage spec to specify how storage shall be used.
	Storage *StorageSpec `json:"storage,omitempty"`
	// Volumes allows configuration of additional volumes on the output StatefulSet definition. Volumes specified will
	// be appended to other volumes that are generated as a result of StorageSpec objects.
	Volumes []v1.Volume `json:"volumes,omitempty"`
	// ObjectStorageConfig configures object storage in Thanos.
	ObjectStorageConfig *v1.SecretKeySelector `json:"objectStorageConfig,omitempty"`
	// ListenLocal makes the Thanos ruler listen on loopback, so that it
	// does not bind against the Pod IP.
	ListenLocal bool `json:"listenLocal,omitempty"`
	// QueryEndpoints defines Thanos querier endpoints from which to query metrics.
	// Maps to the --query flag of thanos ruler.
	QueryEndpoints []string `json:"queryEndpoints"`
	// Define URL to send alerts to alertmanager.  For Thanos v0.10.0 and higher,
	// AlertManagersConfig should be used instead.
	// Maps to the `alertmanagers.url` arg.
	AlertManagersURL string `json:"alertmanagersUrl,omitempty"`
	// Define configuration for connecting to alertmanager.  Only available with thanos v0.10.0
	// and higher.  Maps to the `alertmanagers.config` arg.
	AlertManagersConfig *v1.SecretKeySelector `json:"alertmanagersConfig,omitempty"`
	// A label selector to select which PrometheusRules to mount for alerting and
	// recording.
	RuleSelector *metav1.LabelSelector `json:"ruleSelector,omitempty"`
	// Namespaces to be selected for Rules discovery. If unspecified, only
	// the same namespace as the ThanosRuler object is in is used.
	RuleNamespaceSelector *metav1.LabelSelector `json:"ruleNamespaceSelector,omitempty"`
	// EnforcedNamespaceLabel enforces adding a namespace label of origin for each alert
	// and metric that is user created. The label value will always be the namespace of the object that is
	// being created.
	EnforcedNamespaceLabel string `json:"enforcedNamespaceLabel,omitempty"`
	// Log level for ThanosRuler to be configured with.
	LogLevel string `json:"logLevel,omitempty"`
	// Log format for ThanosRuler to be configured with.
	LogFormat string `json:"logFormat,omitempty"`
	// Port name used for the pods and governing service.
	// This defaults to web
	PortName string `json:"portName,omitempty"`
	// Interval between consecutive evaluations.
	EvaluationInterval string `json:"evaluationInterval,omitempty"`
	// Time duration ThanosRuler shall retain data for. Default is '24h',
	// and must match the regular expression `[0-9]+(ms|s|m|h|d|w|y)` (milliseconds seconds minutes hours days weeks years).
	Retention string `json:"retention,omitempty"`
	// Containers allows injecting additional containers or modifying operator generated
	// containers. This can be used to allow adding an authentication proxy to a ThanosRuler pod or
	// to change the behavior of an operator generated container. Containers described here modify
	// an operator generated container if they share the same name and modifications are done via a
	// strategic merge patch. The current container names are: `thanos-ruler` and `rules-configmap-reloader`.
	// Overriding containers is entirely outside the scope of what the maintainers will support and by doing
	// so, you accept that this behaviour may break at any time without notice.
	Containers []v1.Container `json:"containers,omitempty"`
	// InitContainers allows adding initContainers to the pod definition. Those can be used to e.g.
	// fetch secrets for injection into the ThanosRuler configuration from external sources. Any
	// errors during the execution of an initContainer will lead to a restart of the Pod.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
	// Using initContainers for any use case other then secret fetching is entirely outside the scope
	// of what the maintainers will support and by doing so, you accept that this behaviour may break
	// at any time without notice.
	InitContainers []v1.Container `json:"initContainers,omitempty"`
	// TracingConfig configures tracing in Thanos. This is an experimental feature, it may change in any upcoming release in a breaking way.
	TracingConfig *v1.SecretKeySelector `json:"tracingConfig,omitempty"`
	// Labels configure the external label pairs to ThanosRuler. If not provided, default replica label
	// `thanos_ruler_replica` will be added as a label and be dropped in alerts.
	Labels map[string]string `json:"labels,omitempty"`
	// AlertDropLabels configure the label names which should be dropped in ThanosRuler alerts.
	// If `labels` field is not provided, `thanos_ruler_replica` will be dropped in alerts by default.
	AlertDropLabels []string `json:"alertDropLabels,omitempty"`
}

// ThanosRulerStatus is the most recent observed status of the ThanosRuler. Read-only. Not
// included when requesting from the apiserver, only from the Prometheus
// Operator API itself. More info:
// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
// +k8s:openapi-gen=true
type ThanosRulerStatus struct {
	// Represents whether any actions on the underlying managed objects are
	// being performed. Only delete actions will be performed.
	Paused bool `json:"paused"`
	// Total number of non-terminated pods targeted by this ThanosRuler deployment
	// (their labels match the selector).
	Replicas int32 `json:"replicas"`
	// Total number of non-terminated pods targeted by this ThanosRuler deployment
	// that have the desired version spec.
	UpdatedReplicas int32 `json:"updatedReplicas"`
	// Total number of available pods (ready for at least minReadySeconds)
	// targeted by this ThanosRuler deployment.
	AvailableReplicas int32 `json:"availableReplicas"`
	// Total number of unavailable pods targeted by this ThanosRuler deployment.
	UnavailableReplicas int32 `json:"unavailableReplicas"`
}

// DeepCopyObject implements the runtime.Object interface.
func (l *ThanosRuler) DeepCopyObject() runtime.Object {
	return l.DeepCopy()
}

// DeepCopyObject implements the runtime.Object interface.
func (l *ThanosRulerList) DeepCopyObject() runtime.Object {
	return l.DeepCopy()
}
