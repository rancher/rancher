package v1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster,path=clusterrepos
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRepo represents a particular helm repository and also contains
// details about its location and credentials to connect to it for fetching
// charts hosted in that particular helm repository.
type ClusterRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Contains details about the helm repository that needs to be used.
	// More info: kubectl explain clusterrepo.spec
	Spec RepoSpec `json:"spec"`

	// Contains details of the helm repository that is currently being used in the cluster.
	// More info: kubectl explain clusterrepo.status
	// +optional
	Status RepoStatus `json:"status,omitempty"`
}

// SecretReference references to a secret object which contains the credentials.
type SecretReference struct {
	// Name is the name of the secret.
	Name string `json:"name,omitempty"`

	// Namespace is the namespace where the secret resides.
	Namespace string `json:"namespace,omitempty"`
}

// RepoSpec contains details about the helm repository that needs to be used.
type RepoSpec struct {
	// URL is the http URL of the helm repository to connect to.
	URL string `json:"url,omitempty"`

	// GitRepo is the git repo to clone which contains the helm repository.
	GitRepo string `json:"gitRepo,omitempty"`

	// GitBranch is the git branch where the helm repository is hosted.
	GitBranch string `json:"gitBranch,omitempty"`

	// CABundle is a PEM encoded CA bundle which will be used to validate the repo's certificate.
	// If unspecified, system trust roots will be used.
	CABundle []byte `json:"caBundle,omitempty"`

	// InsecureSkipTLSverify will disable the TLS verification when downloading the helm repository's index file.
	// Defaults is false. Enabling this is not recommended for production due to the security implications.
	InsecureSkipTLSverify bool `json:"insecureSkipTLSVerify,omitempty"`

	// ClientSecret is the client secret to be used when connecting to a helm repository.
	// The expected secret type is "kubernetes.io/basic-auth" or "kubernetes.io/tls" for Helm repositories
	// and "kubernetes.io/basic-auth" or "kubernetes.io/ssh-auth" for Github helm repositories.
	ClientSecret *SecretReference `json:"clientSecret,omitempty"`

	// BasicAuthSecretName is the client secret to be used to connect to the helm repository.
	BasicAuthSecretName string `json:"basicAuthSecretName,omitempty"`

	// ForceUpdate will cause the helm repository index file stored in Rancher
	// to be updated from the Helm repository URL. This means if there are changes
	// in the helm repository they will be pulled into Rancher manager.
	ForceUpdate *metav1.Time `json:"forceUpdate,omitempty"`

	// ServiceAccount when specified will be used in creating helm operation pods which in turn
	// run the helm install or uninstall commands for a chart.
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// ServiceAccountNamespace is the namespace of the service account to use.
	ServiceAccountNamespace string `json:"serviceAccountNamespace,omitempty"`

	// If disabled the repo clone will not be updated or allowed to be installed from.
	Enabled *bool `json:"enabled,omitempty"`

	// DisableSameOriginCheck if true attaches the Basic Auth Header to all helm client API calls
	// regardless of whether the destination of the API call matches the origin of the repository's URL.
	// Defaults to false, which keeps the SameOrigin check enabled. Setting this to true is not recommended
	// in production environments due to the security implications.
	DisableSameOriginCheck bool `json:"disableSameOriginCheck,omitempty"`
}

type RepoCondition string

const (
	RepoDownloaded         RepoCondition = "Downloaded"
	FollowerRepoDownloaded RepoCondition = "FollowerDownloaded"
)

// RepoStatus contains details of the helm repository that is currently being used in the cluster.
type RepoStatus struct {
	// ObservedGeneration is used by Rancher controller to track the latest generation of the resource that it triggered on.
	ObservedGeneration int64 `json:"observedGeneration"`

	// IndexConfigMapName is the name of the configmap which stores the helm repository index.
	IndexConfigMapName string `json:"indexConfigMapName,omitempty"`

	// IndexConfigMapNamespace is the namespace of the helm repository index configmap in which it resides.
	IndexConfigMapNamespace string `json:"indexConfigMapNamespace,omitempty"`

	// IndexConfigMapResourceVersion is the resourceversion of the helm repository index configmap.
	IndexConfigMapResourceVersion string `json:"indexConfigMapResourceVersion,omitempty"`

	// DownloadTime is the time when the index was last downloaded.
	DownloadTime metav1.Time `json:"downloadTime,omitempty"`

	// URL used for fetching the helm repository index file.
	URL string `json:"url,omitempty"`

	// Branch is the Git branch in the git repository used to fetch the helm repository.
	Branch string `json:"branch,omitempty"`

	// Commit is the latest commit in the cloned git repository by Rancher.
	Commit string `json:"commit,omitempty"`

	// Conditions contain information about when the status conditions were updated and
	// to what.
	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Operation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            OperationStatus `json:"status"`
}

type OperationStatus struct {
	ObservedGeneration int64                               `json:"observedGeneration"`
	Action             string                              `json:"action,omitempty"`
	Chart              string                              `json:"chart,omitempty"`
	Version            string                              `json:"version,omitempty"`
	Release            string                              `json:"releaseName,omitempty"`
	Namespace          string                              `json:"namespace,omitempty"`
	ProjectID          string                              `json:"projectId,omitempty"`
	Token              string                              `json:"token,omitempty"`
	Command            []string                            `json:"command,omitempty"`
	PodName            string                              `json:"podName,omitempty"`
	PodNamespace       string                              `json:"podNamespace,omitempty"`
	PodCreated         bool                                `json:"podCreated,omitempty"`
	Conditions         []genericcondition.GenericCondition `json:"conditions,omitempty"`
}
