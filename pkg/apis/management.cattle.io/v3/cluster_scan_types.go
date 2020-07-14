package v3

import (
	"github.com/rancher/norman/condition"
	"github.com/rancher/norman/types"
	"github.com/rancher/rke/types/kdm"
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

type CisScanConfig struct {
	// IDs of the checks that need to be skipped in the final report
	OverrideSkip []string `json:"overrideSkip"`
	// Override the CIS benchmark version to use for the scan (instead of latest)
	OverrideBenchmarkVersion string `json:"overrideBenchmarkVersion,omitempty"`
	// scan profile to use
	Profile CisScanProfileType `json:"profile,omitempty" norman:"required,options=permissive|hardened,default=permissive"`
	// Internal flag for debugging master component of the scan
	DebugMaster bool `json:"debugMaster"`
	// Internal flag for debugging worker component of the scan
	DebugWorker bool `json:"debugWorker"`
}

type CisScanStatus struct {
	Total         int `json:"total"`
	Pass          int `json:"pass"`
	Fail          int `json:"fail"`
	Skip          int `json:"skip"`
	NotApplicable int `json:"notApplicable"`
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
	// Run type
	RunType ClusterScanRunType `json:"runType,omitempty"`
	// scanConfig
	ScanConfig ClusterScanConfig `yaml:",omitempty" json:"scanConfig,omitempty"`
}

type ClusterScanStatus struct {
	Conditions    []ClusterScanCondition `json:"conditions"`
	CisScanStatus *CisScanStatus         `json:"cisScanStatus"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterScan struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterScanSpec   `json:"spec"`
	Status ClusterScanStatus `yaml:"status" json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CisConfig struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Params kdm.CisConfigParams `yaml:"params" json:"params,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CisBenchmarkVersion struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Info kdm.CisBenchmarkVersionInfo `json:"info" yaml:"info"`
}

type ScheduledClusterScanConfig struct {
	// Cron Expression for Schedule
	CronSchedule string `yaml:"cron_schedule" json:"cronSchedule,omitempty"`
	// Number of past scans to keep
	Retention int `yaml:"retention" json:"retention,omitempty"`
}

type ScheduledClusterScan struct {
	// Enable or disable scheduled scans
	Enabled        bool                        `yaml:"enabled" json:"enabled,omitempty" norman:"default=false"`
	ScheduleConfig *ScheduledClusterScanConfig `yaml:"schedule_config" json:"scheduleConfig,omitempty"`
	ScanConfig     *ClusterScanConfig          `yaml:"scan_config,omitempty" json:"scanConfig,omitempty"`
}

type ScheduledClusterScanStatus struct {
	Enabled          bool   `yaml:"enabled" json:"enabled,omitempty"`
	LastRunTimestamp string `yaml:"last_run_timestamp" json:"lastRunTimestamp"`
}
