package v1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RepoSpec   `json:"spec"`
	Status            RepoStatus `json:"status"`
}

// SecretReference a reference to a secret object
type SecretReference struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

type RepoSpec struct {
	// URL A http URL of the repo to connect to
	URL string `json:"url,omitempty"`

	// GitRepo a git repo to clone and index as the helm repo
	GitRepo string `json:"gitRepo,omitempty"`

	// GitBranch The git branch to follow
	GitBranch string `json:"gitBranch,omitempty"`

	// CABundle is a PEM encoded CA bundle which will be used to validate the repo's certificate.
	// If unspecified, system trust roots will be used.
	CABundle []byte `json:"caBundle,omitempty"`

	// InsecureSkipTLSverify will use insecure HTTPS to download the repo's index.
	InsecureSkipTLSverify bool `json:"insecureSkipTLSVerify,omitempty"`

	// ClientSecretName is the client secret to be used to connect to the repo
	// It is expected the secret be of type "kubernetes.io/basic-auth" or "kubernetes.io/tls" for Helm repos
	// and "kubernetes.io/basic-auth" or "kubernetes.io/ssh-auth" for git repos.
	// For a repo the Namespace file will be ignored
	ClientSecret *SecretReference `json:"clientSecret,omitempty"`

	// BasicAuthSecretName is the client secret to be used to connect to the repo
	BasicAuthSecretName string `json:"basicAuthSecretName,omitempty"`

	// ForceUpdate will cause the repo index to be downloaded if it was last download before the specified time
	// If ForceUpdate is greater than time.Now() it will not trigger an update
	ForceUpdate *metav1.Time `json:"forceUpdate,omitempty"`

	// ServiceAccount this service account will be used to deploy charts instead of the end users credentials
	ServiceAccount string `json:"serviceAccount,omitempty"`

	// ServiceAccountNamespace namespace of the service account to use. This value is used only on
	// ClusterRepo and will be ignored on Repo
	ServiceAccountNamespace string `json:"serviceAccountNamespace,omitempty"`

	// If disabled the repo clone will not be updated or allowed to be installed from
	Enabled *bool `json:"enabled,omitempty"`

	// DisableSameOriginCheck attaches the Basic Auth Header to all helm client API calls, regardless of whether the destination of the API call matches the origin of the repository's URL
	DisableSameOriginCheck bool `json:"disableSameOriginCheck,omitempty"`
}

type RepoCondition string

const (
	RepoDownloaded         RepoCondition = "Downloaded"
	FollowerRepoDownloaded RepoCondition = "FollowerDownloaded"
)

type RepoStatus struct {
	ObservedGeneration int64 `json:"observedGeneration"`

	// IndexConfigMapName is the configmap with the store index in it
	IndexConfigMapName            string `json:"indexConfigMapName,omitempty"`
	IndexConfigMapNamespace       string `json:"indexConfigMapNamespace,omitempty"`
	IndexConfigMapResourceVersion string `json:"indexConfigMapResourceVersion,omitempty"`

	// DownloadTime the time when the index was last downloaded
	DownloadTime metav1.Time `json:"downloadTime,omitempty"`

	// The URL used for the last successful index
	URL string `json:"url,omitempty"`

	// The branch used for the last successful index
	Branch string `json:"branch,omitempty"`

	// The git commit used to generate the index
	Commit string `json:"commit,omitempty"`

	Conditions []genericcondition.GenericCondition `json:"conditions,omitempty"`
}

// +genclient
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
