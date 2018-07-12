package client

const (
	ContainerType                          = "container"
	ContainerFieldAllowPrivilegeEscalation = "allowPrivilegeEscalation"
	ContainerFieldCapAdd                   = "capAdd"
	ContainerFieldCapDrop                  = "capDrop"
	ContainerFieldCommand                  = "command"
	ContainerFieldEntrypoint               = "entrypoint"
	ContainerFieldEnvironment              = "environment"
	ContainerFieldEnvironmentFrom          = "environmentFrom"
	ContainerFieldExitCode                 = "exitCode"
	ContainerFieldImage                    = "image"
	ContainerFieldImagePullPolicy          = "imagePullPolicy"
	ContainerFieldInitContainer            = "initContainer"
	ContainerFieldLivenessProbe            = "livenessProbe"
	ContainerFieldName                     = "name"
	ContainerFieldPorts                    = "ports"
	ContainerFieldPostStart                = "postStart"
	ContainerFieldPreStop                  = "preStop"
	ContainerFieldPrivileged               = "privileged"
	ContainerFieldReadOnly                 = "readOnly"
	ContainerFieldReadinessProbe           = "readinessProbe"
	ContainerFieldResources                = "resources"
	ContainerFieldRestartCount             = "restartCount"
	ContainerFieldRunAsGroup               = "runAsGroup"
	ContainerFieldRunAsNonRoot             = "runAsNonRoot"
	ContainerFieldState                    = "state"
	ContainerFieldStdin                    = "stdin"
	ContainerFieldStdinOnce                = "stdinOnce"
	ContainerFieldTTY                      = "tty"
	ContainerFieldTerminationMessagePath   = "terminationMessagePath"
	ContainerFieldTerminationMessagePolicy = "terminationMessagePolicy"
	ContainerFieldTransitioning            = "transitioning"
	ContainerFieldTransitioningMessage     = "transitioningMessage"
	ContainerFieldUid                      = "uid"
	ContainerFieldVolumeDevices            = "volumeDevices"
	ContainerFieldVolumeMounts             = "volumeMounts"
	ContainerFieldWorkingDir               = "workingDir"
)

type Container struct {
	AllowPrivilegeEscalation *bool                 `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	CapAdd                   []string              `json:"capAdd,omitempty" yaml:"capAdd,omitempty"`
	CapDrop                  []string              `json:"capDrop,omitempty" yaml:"capDrop,omitempty"`
	Command                  []string              `json:"command,omitempty" yaml:"command,omitempty"`
	Entrypoint               []string              `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Environment              map[string]string     `json:"environment,omitempty" yaml:"environment,omitempty"`
	EnvironmentFrom          []EnvironmentFrom     `json:"environmentFrom,omitempty" yaml:"environmentFrom,omitempty"`
	ExitCode                 *int64                `json:"exitCode,omitempty" yaml:"exitCode,omitempty"`
	Image                    string                `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullPolicy          string                `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	InitContainer            bool                  `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	LivenessProbe            *Probe                `json:"livenessProbe,omitempty" yaml:"livenessProbe,omitempty"`
	Name                     string                `json:"name,omitempty" yaml:"name,omitempty"`
	Ports                    []ContainerPort       `json:"ports,omitempty" yaml:"ports,omitempty"`
	PostStart                *Handler              `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PreStop                  *Handler              `json:"preStop,omitempty" yaml:"preStop,omitempty"`
	Privileged               *bool                 `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	ReadOnly                 *bool                 `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	ReadinessProbe           *Probe                `json:"readinessProbe,omitempty" yaml:"readinessProbe,omitempty"`
	Resources                *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
	RestartCount             int64                 `json:"restartCount,omitempty" yaml:"restartCount,omitempty"`
	RunAsGroup               *int64                `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool                 `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	State                    string                `json:"state,omitempty" yaml:"state,omitempty"`
	Stdin                    bool                  `json:"stdin,omitempty" yaml:"stdin,omitempty"`
	StdinOnce                bool                  `json:"stdinOnce,omitempty" yaml:"stdinOnce,omitempty"`
	TTY                      bool                  `json:"tty,omitempty" yaml:"tty,omitempty"`
	TerminationMessagePath   string                `json:"terminationMessagePath,omitempty" yaml:"terminationMessagePath,omitempty"`
	TerminationMessagePolicy string                `json:"terminationMessagePolicy,omitempty" yaml:"terminationMessagePolicy,omitempty"`
	Transitioning            string                `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage     string                `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uid                      *int64                `json:"uid,omitempty" yaml:"uid,omitempty"`
	VolumeDevices            []VolumeDevice        `json:"volumeDevices,omitempty" yaml:"volumeDevices,omitempty"`
	VolumeMounts             []VolumeMount         `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	WorkingDir               string                `json:"workingDir,omitempty" yaml:"workingDir,omitempty"`
}
