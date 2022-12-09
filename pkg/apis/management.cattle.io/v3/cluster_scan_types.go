package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterScanRunType string
type CisScanProfileType string

const (
	ClusterScanConditionCreated      condition.Cond = Created
	ClusterScanConditionRunCompleted condition.Cond = RunCompleted
	ClusterScanConditionCompleted    condition.Cond = Completed
	ClusterScanConditionFailed       condition.Cond = Failed
	ClusterScanConditionAlerted      condition.Cond = Alerted

	ClusterScanTypeCis         = "cis"
	DefaultNamespaceForCis     = "security-scan"
	DefaultSonobuoyPodName     = "security-scan-runner"
	ConfigMapNameForUserConfig = "security-scan-cfg"

	SonobuoyCompletionAnnotation = "field.cattle.io/sonobuoyDone"
	CisHelmChartOwner            = "field.cattle.io/clusterScanOwner"

	ClusterScanRunTypeManual    ClusterScanRunType = "manual"
	ClusterScanRunTypeScheduled ClusterScanRunType = "scheduled"

	CisScanProfileTypePermissive CisScanProfileType = "permissive"
	CisScanProfileTypeHardened   CisScanProfileType = "hardened"

	DefaultScanOutputFileName string = "output.json"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScan struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec"`
	Status ClusterScanStatus `yaml:"status" json:"status,omitempty"`
}

type ClusterScanSpec struct {
	ScanType string `json:"scanType"`
	// cluster ID
	ClusterID string `json:"clusterId,omitempty" norman:"required,type=reference[cluster]"`
	// Run type
	RunType ClusterScanRunType `json:"runType,omitempty"`
	// scanConfig
	ScanConfig ClusterScanConfig `yaml:",omitempty" json:"scanConfig,omitempty"`
}

type ClusterScanConfig struct {
}

type ClusterScanStatus struct {
	Conditions []ClusterScanCondition `json:"conditions"`
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
