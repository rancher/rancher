package v1

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReleaseSpec   `json:"spec,omitempty"`
	Status ReleaseStatus `json:"status,omitempty"`
}

type ReleaseStatus struct {
	Summary            Summary `json:"summary,omitempty"`
	ObservedGeneration int64   `json:"observedGeneration"`
}

type Summary struct {
	State         string `json:"state,omitempty"`
	Transitioning bool   `json:"transitioning,omitempty"`
	Error         bool   `json:"error,omitempty"`
}

type ReleaseSpec struct {
	// Name is the name of the release
	Name string `json:"name,omitempty"`
	// Info provides information about a release
	Info *Info `json:"info,omitempty"`
	// Chart is the chart that was released.
	Chart *Chart `json:"chart,omitempty"`
	// Config is the set of extra Values added to the chart.
	// These values override the default values inside of the chart.
	Values v3.MapStringInterface `json:"values,omitempty"`
	// Manifest is the string representation of the rendered template.
	Resources []ReleaseResource `json:"resources,omitempty"`
	// Version is an int which represents the version of the release.
	Version int `json:"version,omitempty"`
	// Namespace is the kubernetes namespace of the release.
	Namespace string `json:"namespace,omitempty"`

	HelmMajorVersion int `json:"helmVersion,omitempty"`
}

type ReleaseResource struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
}

// Chart is a helm package that contains metadata, a default config, zero or more
// optionally parameterizable templates, and zero or more charts (dependencies).
type Chart struct {
	// Metadata is the contents of the Chartfile.
	Metadata *Metadata `json:"metadata,omitempty"`
	// Values are default config for this chart.
	Values v3.MapStringInterface `json:"values,omitempty"`
}

// Metadata for a Chart file. This models the structure of a Chart.yaml file.
type Metadata struct {
	// The name of the chart
	Name string `json:"name,omitempty"`
	// The URL to a relevant project page, git repo, or contact person
	Home string `json:"home,omitempty"`
	// Source is the URL to the source code of this chart
	Sources []string `json:"sources,omitempty"`
	// A SemVer 2 conformant version string of the chart
	Version string `json:"version,omitempty"`
	// A one-sentence description of the chart
	Description string `json:"description,omitempty"`
	// A list of string keywords
	Keywords []string `json:"keywords,omitempty"`
	// A list of name and URL/email address combinations for the maintainer(s)
	Maintainers []Maintainer `json:"maintainers,omitempty"`
	// The URL to an icon file.
	Icon string `json:"icon,omitempty"`
	// The API Version of this chart.
	APIVersion string `json:"apiVersion,omitempty"`
	// The condition to check to enable chart
	Condition string `json:"condition,omitempty"`
	// The tags to check to enable chart
	Tags string `json:"tags,omitempty"`
	// The version of the application enclosed inside of this chart.
	AppVersion string `json:"appVersion,omitempty"`
	// Whether or not this chart is deprecated
	Deprecated bool `json:"deprecated,omitempty"`
	// Annotations are additional mappings uninterpreted by Helm,
	// made available for inspection by other applications.
	Annotations map[string]string `json:"annotations,omitempty"`
	// KubeVersion is a SemVer constraint specifying the version of Kubernetes required.
	KubeVersion string `json:"kubeVersion,omitempty"`
	// Specifies the chart type: application or library
	Type string `json:"type,omitempty"`
}

// Maintainer describes a Chart maintainer.
type Maintainer struct {
	// Name is a user name or organization name
	Name string `json:"name,omitempty"`
	// Email is an optional email address to contact the named maintainer
	Email string `json:"email,omitempty"`
	// URL is an optional URL to an address for the named maintainer
	URL string `json:"url,omitempty"`
}

// Info describes release information.
type Info struct {
	// FirstDeployed is when the release was first deployed.
	FirstDeployed *metav1.Time `json:"firstDeployed,omitempty"`
	// LastDeployed is when the release was last deployed.
	LastDeployed *metav1.Time `json:"lastDeployed,omitempty"`
	// Deleted tracks when this object was deleted.
	Deleted *metav1.Time `json:"deleted,omitempty"`
	// Description is human-friendly "log entry" about this release.
	Description string `json:"description,omitempty"`
	// Status is the current state of the release
	Status Status `json:"status,omitempty" wrangler:"options=unknown|deployed|uninstalled|superseded|failed|uninstalling|pending-install|pending-upgrade|pending-rollback"`
	// Contains the rendered templates/NOTES.txt if available
	Notes  string `json:"notes,omitempty"`
	Readme string `json:"readme,omitempty"`
}

type Status string

// Describe the status of a release
// NOTE: Make sure to update cmd/helm/status.go when adding or modifying any of these statuses.
const (
	// StatusUnknown indicates that a release is in an uncertain state.
	StatusUnknown Status = "unknown"
	// StatusDeployed indicates that the release has been pushed to Kubernetes.
	StatusDeployed Status = "deployed"
	// StatusUninstalled indicates that a release has been uninstalled from Kubernetes.
	StatusUninstalled Status = "uninstalled"
	// StatusSuperseded indicates that this release object is outdated and a newer one exists.
	StatusSuperseded Status = "superseded"
	// StatusFailed indicates that the release was not successfully deployed.
	StatusFailed Status = "failed"
	// StatusUninstalling indicates that a uninstall operation is underway.
	StatusUninstalling Status = "uninstalling"
	// StatusPendingInstall indicates that an install operation is underway.
	StatusPendingInstall Status = "pending-install"
	// StatusPendingUpgrade indicates that an upgrade operation is underway.
	StatusPendingUpgrade Status = "pending-upgrade"
	// StatusPendingRollback indicates that an rollback operation is underway.
	StatusPendingRollback Status = "pending-rollback"
)
