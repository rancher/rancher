package client

const (
	EphemeralContainerType                          = "ephemeralContainer"
	EphemeralContainerFieldArgs                     = "args"
	EphemeralContainerFieldCommand                  = "command"
	EphemeralContainerFieldEnv                      = "env"
	EphemeralContainerFieldEnvFrom                  = "envFrom"
	EphemeralContainerFieldImage                    = "image"
	EphemeralContainerFieldImagePullPolicy          = "imagePullPolicy"
	EphemeralContainerFieldLifecycle                = "lifecycle"
	EphemeralContainerFieldLivenessProbe            = "livenessProbe"
	EphemeralContainerFieldName                     = "name"
	EphemeralContainerFieldPorts                    = "ports"
	EphemeralContainerFieldReadinessProbe           = "readinessProbe"
	EphemeralContainerFieldResources                = "resources"
	EphemeralContainerFieldSecurityContext          = "securityContext"
	EphemeralContainerFieldStartupProbe             = "startupProbe"
	EphemeralContainerFieldStdin                    = "stdin"
	EphemeralContainerFieldStdinOnce                = "stdinOnce"
	EphemeralContainerFieldTTY                      = "tty"
	EphemeralContainerFieldTargetContainerName      = "targetContainerName"
	EphemeralContainerFieldTerminationMessagePath   = "terminationMessagePath"
	EphemeralContainerFieldTerminationMessagePolicy = "terminationMessagePolicy"
	EphemeralContainerFieldVolumeDevices            = "volumeDevices"
	EphemeralContainerFieldVolumeMounts             = "volumeMounts"
	EphemeralContainerFieldWorkingDir               = "workingDir"
)

type EphemeralContainer struct {
	Args                     []string              `json:"args,omitempty" yaml:"args,omitempty"`
	Command                  []string              `json:"command,omitempty" yaml:"command,omitempty"`
	Env                      []EnvVar              `json:"env,omitempty" yaml:"env,omitempty"`
	EnvFrom                  []EnvFromSource       `json:"envFrom,omitempty" yaml:"envFrom,omitempty"`
	Image                    string                `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullPolicy          string                `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Lifecycle                *Lifecycle            `json:"lifecycle,omitempty" yaml:"lifecycle,omitempty"`
	LivenessProbe            *Probe                `json:"livenessProbe,omitempty" yaml:"livenessProbe,omitempty"`
	Name                     string                `json:"name,omitempty" yaml:"name,omitempty"`
	Ports                    []ContainerPort       `json:"ports,omitempty" yaml:"ports,omitempty"`
	ReadinessProbe           *Probe                `json:"readinessProbe,omitempty" yaml:"readinessProbe,omitempty"`
	Resources                *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	SecurityContext          *SecurityContext      `json:"securityContext,omitempty" yaml:"securityContext,omitempty"`
	StartupProbe             *Probe                `json:"startupProbe,omitempty" yaml:"startupProbe,omitempty"`
	Stdin                    bool                  `json:"stdin,omitempty" yaml:"stdin,omitempty"`
	StdinOnce                bool                  `json:"stdinOnce,omitempty" yaml:"stdinOnce,omitempty"`
	TTY                      bool                  `json:"tty,omitempty" yaml:"tty,omitempty"`
	TargetContainerName      string                `json:"targetContainerName,omitempty" yaml:"targetContainerName,omitempty"`
	TerminationMessagePath   string                `json:"terminationMessagePath,omitempty" yaml:"terminationMessagePath,omitempty"`
	TerminationMessagePolicy string                `json:"terminationMessagePolicy,omitempty" yaml:"terminationMessagePolicy,omitempty"`
	VolumeDevices            []VolumeDevice        `json:"volumeDevices,omitempty" yaml:"volumeDevices,omitempty"`
	VolumeMounts             []VolumeMount         `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	WorkingDir               string                `json:"workingDir,omitempty" yaml:"workingDir,omitempty"`
}
