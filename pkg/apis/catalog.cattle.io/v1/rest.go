package v1

import (
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
)

type ChartInstallAction struct {
	Atomic                   bool                  `json:"atomic,omitempty"`
	ChartName                string                `json:"chartName,omitempty"`
	Version                  string                `json:"version,omitempty"`
	DryRun                   bool                  `json:"dryRun,omitempty"`
	DisableHooks             bool                  `json:"noHooks,omitempty"`
	Wait                     bool                  `json:"wait,omitempty"`
	Timeout                  time.Duration         `json:"timeout,omitempty"`
	Namespace                string                `json:"namespace,omitempty"`
	ReleaseName              string                `json:"releaseName,omitempty"`
	GenerateName             bool                  `json:"generateName,omitempty"`
	NameTemplate             string                `json:"nameTemplate,omitempty"`
	Description              string                `json:"description,omitempty"`
	SkipCRDs                 bool                  `json:"skipCRDs,omitempty"`
	DisableOpenAPIValidation bool                  `json:"disableOpenAPIValidation,omitempty"`
	Values                   v3.MapStringInterface `json:"values,omitempty"`
}

type ChartInfo struct {
	Readme    string                `json:"readme,omitempty"`
	Values    v3.MapStringInterface `json:"values,omitempty"`
	Questions v3.MapStringInterface `json:"questions,omitempty"`
	Chart     v3.MapStringInterface `json:"chart,omitempty"`
}

type ChartUninstallAction struct {
	DisableHooks bool          `json:"noHooks,omitempty"`
	DryRun       bool          `json:"dryRun,omitempty"`
	KeepHistory  bool          `json:"keepHistory,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty"`
	Description  string        `json:"description,omitempty"`
}

type ChartUpgradeAction struct {
	Atomic        bool                  `json:"atomic,omitempty"`
	ChartName     string                `json:"chartName,omitempty"`
	Version       string                `json:"version,omitempty"`
	Namespace     string                `json:"namespace,omitempty"`
	ReleaseName   string                `json:"releaseName,omitempty"`
	Timeout       time.Duration         `json:"timeout,omitempty"`
	Wait          bool                  `json:"wait,omitempty"`
	DisableHooks  bool                  `json:"noHooks,omitempty"`
	DryRun        bool                  `json:"dryRun,omitempty"`
	Force         bool                  `json:"force,omitempty"`
	ResetValues   bool                  `json:"resetValues,omitempty"`
	ReuseValues   bool                  `json:"reuseValues,omitempty"`
	MaxHistory    int                   `json:"historyMax,omitempty"`
	CleanupOnFail bool                  `json:"cleanupOnFail,omitempty"`
	Description   string                `json:"description,omitempty"`
	Values        v3.MapStringInterface `json:"values,omitempty"`
}

type ChartRollbackAction struct {
	Timeout       time.Duration `json:"timeout,omitempty"`
	Wait          bool          `json:"wait,omitempty"`
	DisableHooks  bool          `json:"noHooks,omitempty"`
	DryRun        bool          `json:"dryRun,omitempty"`
	Recreate      bool          `json:"recreatePods,omitempty"`
	Force         bool          `json:"force,omitempty"`
	CleanupOnFail bool          `json:"cleanupOnFail,omitempty"`
}

type ChartActionOutput struct {
	OperationName      string `json:"operationName,omitempty"`
	OperationNamespace string `json:"operationNamespace,omitempty"`
}
