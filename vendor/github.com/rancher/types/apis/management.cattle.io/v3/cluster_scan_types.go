package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	typescond "github.com/rancher/types/condition"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterScanConditionCreated      condition.Cond = typescond.Created
	ClusterScanConditionRunCompleted condition.Cond = typescond.RunCompleted
	ClusterScanConditionCompleted    condition.Cond = typescond.Completed
	ClusterScanConditionFailed       condition.Cond = typescond.Failed

	ClusterScanTypeCis         = "cis"
	DefaultNamespaceForCis     = "security-scan"
	DefaultSonobuoyPodName     = "security-scan-runner"
	ConfigMapNameForUserConfig = "security-scan-cfg"

	RunCisScanAnnotation         = "field.cattle.io/runCisScan"
	SonobuoyCompletionAnnotation = "field.cattle.io/sonobuoyDone"
	CisHelmChartOwner            = "field.cattle.io/clusterScanOwner"
)

type CisScanConfig struct {
	// IDs of the checks that need to be skipped in the final report
	OverrideSkip []string `json:"overrideSkip"`
	// Override the CIS benchmark version to use for the scan (instead of latest)
	OverrideBenchmarkVersion string `json:"overrideBenchmarkVersion,omitempty"`
	// Internal flag for debugging master component of the scan
	DebugMaster bool `json:"debugMaster"`
	// Internal flag for debugging worker component of the scan
	DebugWorker bool `json:"debugWorker"`
}

type ClusterScanConfig struct {
	CisScanConfig *CisScanConfig `json:"cisScanConfig"`
}

type ClusterScanCondition struct {
	// Type of condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition
	Message string `json:"message,omitempty"`
}

type ClusterScanSpec struct {
	ScanType string `json:"scanType"`
	// cluster ID
	ClusterID string `json:"clusterId,omitempty" norman:"required,type=reference[cluster]"`
	// manual flag
	Manual bool `yaml:"manual" json:"manual,omitempty"`
	// scanConfig
	ScanConfig ClusterScanConfig `yaml:",omitempty" json:"scanConfig,omitempty"`
}

type ClusterScanStatus struct {
	Conditions []ClusterScanCondition `json:"conditions"`
}

type ClusterScan struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec"`
	Status ClusterScanStatus `yaml:"status" json:"status,omitempty"`
}

type CisBenchmarkVersionInfo struct {
	MinKubernetesVersion string `yaml:"min_kubernetes_version" json:"minKubernetesVersion"`
}

type CisConfigParams struct {
	BenchmarkVersion string `yaml:"benchmark_version" json:"benchmarkVersion"`
}

type CisConfig struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Params CisConfigParams `yaml:"params" json:"params,omitempty"`
}

type CisBenchmarkVersion struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Info CisBenchmarkVersionInfo `json:"info" yaml:"info"`
}
