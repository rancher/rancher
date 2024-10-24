package charts

import (
	"github.com/rancher/shepherd/extensions/clusters"
)

const (
	// defaultRegistrySettingID is a private constant string that contains the ID of system default registry setting.
	defaultRegistrySettingID = "system-default-registry"
	// serverURLSettingID is a private constant string that contains the ID of server URL setting.
	serverURLSettingID   = "server-url"
	rancherChartsName    = "rancher-charts"
	rancherPartnerCharts = "rancher-partner-charts"
	active               = "active"
)

// InstallOptions is a struct of the required options to install a chart.
type InstallOptions struct {
	Cluster   *clusters.ClusterMeta
	Version   string
	ProjectID string
}

// payloadOpts is a private struct that contains the options for the chart payloads.
// It is used to avoid passing the same options to different functions while using the chart helpers.
type payloadOpts struct {
	InstallOptions
	Name            string
	Namespace       string
	Host            string
	DefaultRegistry string
}

// RancherIstioOpts is a struct of the required options to install Rancher Istio with desired chart values.
type RancherIstioOpts struct {
	IngressGateways bool
	EgressGateways  bool
	Pilot           bool
	Telemetry       bool
	Kiali           bool
	Tracing         bool
	CNI             bool
}

// RancherMonitoringOpts is a struct of the required options to install Rancher Monitoring with desired chart values.
type RancherMonitoringOpts struct {
	IngressNginx      bool `json:"ingressNginx" yaml:"ingressNginx"`
	ControllerManager bool `json:"controllerManager" yaml:"controllerManager"`
	Etcd              bool `json:"etcd" yaml:"etcd"`
	Proxy             bool `json:"proxy" yaml:"proxy"`
	Scheduler         bool `json:"scheduler" yaml:"scheduler"`
}

// RancherLoggingOpts is a struct of the required options to install Rancher Logging with desired chart values.
type RancherLoggingOpts struct {
	AdditionalLoggingSources bool
}

// RancherAlertingOpts is a struct of the required options to install Rancher Alerting Drivers with desired chart values.
type RancherAlertingOpts struct {
	SMS   bool
	Teams bool
}

// GetChartCaseEndpointResult is a struct that GetChartCaseEndpoint helper function returns.
// It contains the boolean for healthy response and the request body.
type GetChartCaseEndpointResult struct {
	Ok   bool
	Body string
}
