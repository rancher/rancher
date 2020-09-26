package types

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ChartInstall struct {
	ChartName   string                `json:"chartName,omitempty"`
	Version     string                `json:"version,omitempty"`
	ReleaseName string                `json:"releaseName,omitempty"`
	Description string                `json:"description,omitempty"`
	Values      v3.MapStringInterface `json:"values,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
}

type ChartInstallAction struct {
	Timeout                  *metav1.Duration `json:"timeout,omitempty"`
	Wait                     bool             `json:"wait,omitempty"`
	DisableHooks             bool             `json:"noHooks,omitempty"`
	DisableOpenAPIValidation bool             `json:"disableOpenAPIValidation,omitempty"`
	Namespace                string           `json:"namespace,omitempty"`
	ProjectID                string           `json:"projectId,omitempty"`

	Charts []ChartInstall `json:"charts,omitempty"`
}

type ChartInfo struct {
	Readme    string                `json:"readme,omitempty"`
	APPReadme string                `json:"appReadme,omitempty"`
	Values    v3.MapStringInterface `json:"values,omitempty"`
	Questions v3.MapStringInterface `json:"questions,omitempty"`
	Chart     v3.MapStringInterface `json:"chart,omitempty"`
}

type ChartUninstallAction struct {
	DisableHooks bool             `json:"noHooks,omitempty"`
	DryRun       bool             `json:"dryRun,omitempty"`
	KeepHistory  bool             `json:"keepHistory,omitempty"`
	Timeout      *metav1.Duration `json:"timeout,omitempty"`
	Description  string           `json:"description,omitempty"`
}

type ChartUpgradeAction struct {
	Timeout                  *metav1.Duration `json:"timeout,omitempty"`
	Wait                     bool             `json:"wait,omitempty"`
	DisableHooks             bool             `json:"noHooks,omitempty"`
	DisableOpenAPIValidation bool             `json:"disableOpenAPIValidation,omitempty"`
	Force                    bool             `json:"force,omitempty"`
	MaxHistory               int              `json:"historyMax,omitempty"`
	Install                  bool             `json:"install,omitempty"`
	Namespace                string           `json:"namespace,omitempty"`
	CleanupOnFail            bool             `json:"cleanupOnFail,omitempty"`
	Charts                   []ChartUpgrade   `json:"charts,omitempty"`
}

type ChartUpgrade struct {
	ChartName   string                `json:"chartName,omitempty"`
	Version     string                `json:"version,omitempty"`
	ReleaseName string                `json:"releaseName,omitempty"`
	Force       bool                  `json:"force,omitempty"`
	ResetValues bool                  `json:"resetValues,omitempty"`
	Description string                `json:"description,omitempty"`
	Values      v3.MapStringInterface `json:"values,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
}

type ChartActionOutput struct {
	OperationName      string `json:"operationName,omitempty"`
	OperationNamespace string `json:"operationNamespace,omitempty"`
}
