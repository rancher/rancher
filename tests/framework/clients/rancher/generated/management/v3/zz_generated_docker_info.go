package client

const (
	DockerInfoType                    = "dockerInfo"
	DockerInfoFieldArchitecture       = "architecture"
	DockerInfoFieldCgroupDriver       = "cgroupDriver"
	DockerInfoFieldDebug              = "debug"
	DockerInfoFieldDockerRootDir      = "dockerRootDir"
	DockerInfoFieldDriver             = "driver"
	DockerInfoFieldExperimentalBuild  = "experimentalBuild"
	DockerInfoFieldHTTPProxy          = "httpProxy"
	DockerInfoFieldHTTPSProxy         = "httpsProxy"
	DockerInfoFieldID                 = "id"
	DockerInfoFieldIndexServerAddress = "indexServerAddress"
	DockerInfoFieldInitBinary         = "initBinary"
	DockerInfoFieldKernelVersion      = "kernelVersion"
	DockerInfoFieldLabels             = "labels"
	DockerInfoFieldLoggingDriver      = "loggingDriver"
	DockerInfoFieldName               = "name"
	DockerInfoFieldNoProxy            = "noProxy"
	DockerInfoFieldOSType             = "osType"
	DockerInfoFieldOperatingSystem    = "operatingSystem"
	DockerInfoFieldSecurityOptions    = "securityOptions"
	DockerInfoFieldServerVersion      = "serverVersion"
)

type DockerInfo struct {
	Architecture       string   `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	CgroupDriver       string   `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`
	Debug              bool     `json:"debug,omitempty" yaml:"debug,omitempty"`
	DockerRootDir      string   `json:"dockerRootDir,omitempty" yaml:"dockerRootDir,omitempty"`
	Driver             string   `json:"driver,omitempty" yaml:"driver,omitempty"`
	ExperimentalBuild  bool     `json:"experimentalBuild,omitempty" yaml:"experimentalBuild,omitempty"`
	HTTPProxy          string   `json:"httpProxy,omitempty" yaml:"httpProxy,omitempty"`
	HTTPSProxy         string   `json:"httpsProxy,omitempty" yaml:"httpsProxy,omitempty"`
	ID                 string   `json:"id,omitempty" yaml:"id,omitempty"`
	IndexServerAddress string   `json:"indexServerAddress,omitempty" yaml:"indexServerAddress,omitempty"`
	InitBinary         string   `json:"initBinary,omitempty" yaml:"initBinary,omitempty"`
	KernelVersion      string   `json:"kernelVersion,omitempty" yaml:"kernelVersion,omitempty"`
	Labels             []string `json:"labels,omitempty" yaml:"labels,omitempty"`
	LoggingDriver      string   `json:"loggingDriver,omitempty" yaml:"loggingDriver,omitempty"`
	Name               string   `json:"name,omitempty" yaml:"name,omitempty"`
	NoProxy            string   `json:"noProxy,omitempty" yaml:"noProxy,omitempty"`
	OSType             string   `json:"osType,omitempty" yaml:"osType,omitempty"`
	OperatingSystem    string   `json:"operatingSystem,omitempty" yaml:"operatingSystem,omitempty"`
	SecurityOptions    []string `json:"securityOptions,omitempty" yaml:"securityOptions,omitempty"`
	ServerVersion      string   `json:"serverVersion,omitempty" yaml:"serverVersion,omitempty"`
}
