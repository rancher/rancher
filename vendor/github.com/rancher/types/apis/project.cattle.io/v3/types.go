package v3

import (
	"github.com/rancher/norman/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WorkloadSpec struct {
	Description  string             `json:"description"`
	DeployConfig DeployConfig       `json:"deployConfig"`
	Template     v1.PodTemplateSpec `json:"template"`
	ServiceLinks []Link             `json:"serviceLinks"`
}

type WorkloadStatus struct {
}

type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WorkloadSpec    `json:"spec"`
	Status            *WorkloadStatus `json:"status"`
}

type DeployConfig struct {
	Scale              int64           `json:"scale"`
	BatchSize          string          `json:"batchSize"`
	DeploymentStrategy *DeployStrategy `json:"deploymentStrategy"`
}

type DeploymentParallelConfig struct {
	StartFirst              bool  `json:"startFirst"`
	MinReadySeconds         int64 `json:"minReadySeconds"`
	ProgressDeadlineSeconds int64 `json:"progressDeadlineSeconds"`
}

type DeploymentJobConfig struct {
	BatchLimit            int64 `json:"batchLimit"`
	ActiveDeadlineSeconds int64 `json:"activeDeadlineSeconds"`
	OnDelete              bool  `json:"onDelete"`
}

type DeploymentOrderedConfig struct {
	PartitionSize int64 `json:"partitionSize"`
	OnDelete      bool  `json:"onDelete"`
}

type DeploymentGlobalConfig struct {
	OnDelete bool `json:"onDelete"`
}

type DeployStrategy struct {
	Kind           string                    `json:"kind"`
	ParallelConfig *DeploymentParallelConfig `json:"parallelConfig"`
	JobConfig      *DeploymentJobConfig      `json:"jobConfig"`
	OrderedConfig  *DeploymentOrderedConfig  `json:"orderedConfig"`
	GlobalConfig   *DeploymentGlobalConfig   `json:"globalConfig"`
}

type Link struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

type ServiceAccountToken struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	AccountName string `json:"accountName"`
	AccountUID  string `json:"accountUid"`
	Description string `json:"description"`
	Token       string `json:"token" norman:"writeOnly"`
	CACRT       string `json:"caCrt"`
}
type NamespacedServiceAccountToken ServiceAccountToken

type DockerCredential struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string                        `json:"description"`
	Registries  map[string]RegistryCredential `json:"registries"`
}
type NamespacedDockerCredential DockerCredential

type RegistryCredential struct {
	Description string `json:"description"`
	Username    string `json:"username"`
	Password    string `json:"password" norman:"writeOnly"`
	Auth        string `json:"auth" norman:"writeOnly"`
}

type Certificate struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string `json:"description"`
	Certs       string `json:"certs"`
	Key         string `json:"key" norman:"writeOnly"`

	CertFingerprint         string   `json:"certFingerprint" norman:"nocreate,noupdate"`
	CN                      string   `json:"cn" norman:"nocreate,noupdate"`
	Version                 string   `json:"version" norman:"nocreate,noupdate"`
	ExpiresAt               string   `json:"expiresAt" norman:"nocreate,noupdate"`
	Issuer                  string   `json:"issuer" norman:"nocreate,noupdate"`
	IssuedAt                string   `json:"issuedAt" norman:"nocreate,noupdate"`
	Algorithm               string   `json:"algorithm" norman:"nocreate,noupdate"`
	SerialNumber            string   `json:"serialNumber" norman:"nocreate,noupdate"`
	KeySize                 string   `json:"keySize" norman:"nocreate,noupdate"`
	SubjectAlternativeNames []string `json:"subjectAlternativeNames" norman:"nocreate,noupdate"`
}
type NamespacedCertificate Certificate

type BasicAuth struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string `json:"description"`
	Username    string `json:"username"`
	Password    string `json:"password" norman:"writeOnly"`
}
type NamespacedBasicAuth BasicAuth

type SSHAuth struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Description string `json:"description"`
	PrivateKey  string `json:"privateKey" norman:"writeOnly"`
	Fingerprint string `json:"certFingerprint" norman:"nocreate,noupdate"`
}
type NamespacedSSHAuth SSHAuth

type PublicEndpoint struct {
	NodeName string `json:"node,omitempty" norman:"type=reference[node],nocreate,noupdate"`
	Address  string `json:"address,omitempty" norman:"nocreate,noupdate"`
	Port     int32  `json:"port,omitempty" norman:"nocreate,noupdate"`
	Protocol string `json:"protocol,omitempty" norman:"nocreate,noupdate"`
	// for node port service
	ServiceName string `json:"service,omitempty" norman:"type=reference[service],nocreate,noupdate"`
	// for host port
	PodName string `json:"pod,omitempty" norman:"type=reference[pod],nocreate,noupdate"`
	//serviceName and podName are mutually exclusive
}
