package v1

import (
	"github.com/rancher/wrangler/v3/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ClusterRepoNameLabel = "catalog.cattle.io/cluster-repo-name"

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster,path=clusterrepos
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="HTTP URL",type=string,JSONPath=`.spec.url`
// +kubebuilder:printcolumn:name="Enabled",type=string,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Git Repo",type=string,JSONPath=`.spec.gitRepo`
// +kubebuilder:printcolumn:name="Git Branch",type=string,JSONPath=`.spec.gitBranch`
// +kubebuilder:printcolumn:name="Download Time",type=string,JSONPath=`.status.downloadTime`
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRepo represents a particular Helm repository. It contains details
// about the chart location and the credentials needed for fetching charts
// hosted in that particular Helm repository.
type ClusterRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// RepoSec contains details about the Helm repository that needs to be used.
	// More info: kubectl explain clusterrepo.spec
	Spec RepoSpec `json:"spec"`

	// RepoStatus contains details of the Helm repository that is currently being used in the cluster.
	// More info: kubectl explain clusterrepo.status
	// +optional
	Status RepoStatus `json:"status,omitempty"`
}

// SecretReference references a Secret object which contains the credentials.
type SecretReference struct {
	// Name is the name of the secret.
	Name string `json:"name,omitempty"`

	// Namespace is the namespace where the secret resides.
	Namespace string `json:"namespace,omitempty"`
}

// ExponentialBackOffValues are values in seconds for the ratelimiting func when OCI registry sends 429 http response code.
type ExponentialBackOffValues struct {
	MinWait    int `json:"minWait,omitempty"`
	MaxWait    int `json:"maxWait,omitempty"`
	MaxRetries int `json:"maxRetries,omitempty"`
}

// RepoSpec contains details about the helm repository that needs to be used.
type RepoSpec struct {
	// URL is the HTTP or OCI URL of the helm repository to connect to.
	URL string `json:"url,omitempty"`

	// InsecurePlainHTTP is only valid for OCI URL's and allows insecure connections to registries without enforcing TLS checks.
	InsecurePlainHTTP bool `json:"insecurePlainHttp,omitempty"`

	// GitRepo is the git repo to clone which contains the helm repository.
	GitRepo string `json:"gitRepo,omitempty"`

	// GitBranch is the git branch where the helm repository is hosted.
	GitBranch string `json:"gitBranch,omitempty"`

	// RefreshInterval is the interval at which the Helm repository should be refreshed.
	RefreshInterval int `json:"refreshInterval,omitempty"`

	// ExponentialBackOffValues are values given to the Rancher manager to handle
	// 429 TOOMANYREQUESTS response code from the OCI registry.
	ExponentialBackOffValues *ExponentialBackOffValues `json:"exponentialBackOffValues,omitempty"`

	// CABundle is a PEM encoded CA bundle which will be used to validate the repo's certificate.
	// If unspecified, system trust roots will be used.
	CABundle []byte `json:"caBundle,omitempty"`

	// InsecureSkipTLSverify will disable the TLS verification when downloading the Helm repository's index file.
	// Defaults is false. Enabling this is not recommended for production due to the security implications.
	InsecureSkipTLSverify bool `json:"insecureSkipTLSVerify,omitempty"`

	// ClientSecret is the client secret to be used when connecting to a Helm repository.
	// The expected secret type is "kubernetes.io/basic-auth" or "kubernetes.io/tls" for HTTP Helm repositories,
	// only "kubernetes.io/basic-auth" for OCI Helm repostories and "kubernetes.io/basic-auth"
	// or "kubernetes.io/ssh-auth" for Github Helm repositories.
	ClientSecret *SecretReference `json:"clientSecret,omitempty"`

	// BasicAuthSecretName is the client secret to be used to connect to the Helm repository.
	BasicAuthSecretName string `json:"basicAuthSecretName,omitempty"`

	// ForceUpdate will cause the Helm repository index file stored in Rancher
	// to be updated from the Helm repository URL. This means if there are changes
	// in the Helm repository they will be pulled into Rancher manager.
	ForceUpdate *metav1.Time `json:"forceUpdate,omitempty"`

	// ServiceAccount when specified will be used in creating Helm operation pods which in turn
	// run the Helm install or uninstall commands for a chart.
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// ServiceAccountNamespace is the namespace of the service account to use.
	ServiceAccountNamespace string `json:"serviceAccountNamespace,omitempty"`

	// If disabled the repo will not be updated and won't pick up new changes.
	Enabled *bool `json:"enabled,omitempty"`

	// DisableSameOriginCheck if true attaches the Basic Auth Header to all Helm client API calls
	// regardless of whether the destination of the API call matches the origin of the repository's URL.
	// Defaults to false, which keeps the SameOrigin check enabled. Setting this to true is not recommended
	// in production environments due to the security implications.
	DisableSameOriginCheck bool `json:"disableSameOriginCheck,omitempty"`
}

type RepoCondition string

const (
	RepoDownloaded         RepoCondition = "Downloaded"
	FollowerRepoDownloaded RepoCondition = "FollowerDownloaded"
	OCIDownloaded          RepoCondition = "OCIDownloaded"
)

// RepoStatus contains details of the Helm repository that is currently being used in the cluster.
type RepoStatus struct {
	// ObservedGeneration is used by Rancher controller to track the latest generation of the resource that it triggered on.
	ObservedGeneration int64 `json:"observedGeneration"`

	// IndexConfigMapName is the name of the configmap which stores the Helm repository index.
	IndexConfigMapName string `json:"indexConfigMapName,omitempty"`

	// IndexConfigMapNamespace is the namespace of the Helm repository index configmap in which it resides.
	IndexConfigMapNamespace string `json:"indexConfigMapNamespace,omitempty"`

	// IndexConfigMapResourceVersion is the resourceversion of the Helm repository index configmap.
	IndexConfigMapResourceVersion string `json:"indexConfigMapResourceVersion,omitempty"`

	// DownloadTime is the time when the index was last downloaded.
	DownloadTime metav1.Time `json:"downloadTime,omitempty"`

	// URL used for fetching the Helm repository index file.
	URL string `json:"url,omitempty"`

	// Branch is the Git branch in the git repository used to fetch the Helm repository.
	Branch string `json:"branch,omitempty"`

	// Commit is the latest commit in the cloned git repository by Rancher.
	Commit string `json:"commit,omitempty"`

	// Conditions contain information about when the status conditions were updated and
	// to what.
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`

	// Number of times the handler will retry if it gets a 429 error
	NumberOfRetries int `json:"numberOfRetries,omitempty"`

	// The time the next retry will happen
	NextRetryAt metav1.Time `json:"nextRetryAt,omitempty"`

	// If the handler should be skipped or not
	ShouldNotSkip bool `json:"shouldNotSkip,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Operation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            OperationStatus `json:"status"`
}

// OperationStatus represents the status of a helm operation that's going to be created
type OperationStatus struct {
	ObservedGeneration     int64                               `json:"observedGeneration"`
	Action                 string                              `json:"action,omitempty"`
	Chart                  string                              `json:"chart,omitempty"`
	Version                string                              `json:"version,omitempty"`
	Release                string                              `json:"releaseName,omitempty"`
	Namespace              string                              `json:"namespace,omitempty"`
	ProjectID              string                              `json:"projectId,omitempty"`
	Token                  string                              `json:"token,omitempty"`
	Command                []string                            `json:"command,omitempty"`
	PodName                string                              `json:"podName,omitempty"`
	PodNamespace           string                              `json:"podNamespace,omitempty"`
	PodCreated             bool                                `json:"podCreated,omitempty"`
	Conditions             []genericcondition.GenericCondition `json:"conditions,omitempty"`
	AutomaticCPTolerations bool                                `json:"automaticCPTolerations,omitempty"`
	Tolerations            []corev1.Toleration                 `json:"tolerations,omitempty"`
}
