package client

const (
	ContainerType                          = "container"
	ContainerFieldAllowPrivilegeEscalation = "allowPrivilegeEscalation"
	ContainerFieldAppArmorProfile          = "appArmorProfile"
	ContainerFieldCapAdd                   = "capAdd"
	ContainerFieldCapDrop                  = "capDrop"
	ContainerFieldCommand                  = "command"
	ContainerFieldEntrypoint               = "entrypoint"
	ContainerFieldEnv                      = "env"
	ContainerFieldEnvFrom                  = "envFrom"
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
	ContainerFieldProcMount                = "procMount"
	ContainerFieldReadOnly                 = "readOnly"
	ContainerFieldReadinessProbe           = "readinessProbe"
	ContainerFieldResizePolicy             = "resizePolicy"
	ContainerFieldResources                = "resources"
	ContainerFieldRestartCount             = "restartCount"
	ContainerFieldRestartPolicy            = "restartPolicy"
	ContainerFieldRunAsGroup               = "runAsGroup"
	ContainerFieldRunAsNonRoot             = "runAsNonRoot"
	ContainerFieldSeccompProfile           = "seccompProfile"
	ContainerFieldStartupProbe             = "startupProbe"
	ContainerFieldState                    = "state"
	ContainerFieldStdin                    = "stdin"
	ContainerFieldStdinOnce                = "stdinOnce"
	ContainerFieldStopSignal               = "stopSignal"
	ContainerFieldTTY                      = "tty"
	ContainerFieldTerminationMessagePath   = "terminationMessagePath"
	ContainerFieldTerminationMessagePolicy = "terminationMessagePolicy"
	ContainerFieldTransitioning            = "transitioning"
	ContainerFieldTransitioningMessage     = "transitioningMessage"
	ContainerFieldUid                      = "uid"
	ContainerFieldVolumeDevices            = "volumeDevices"
	ContainerFieldVolumeMounts             = "volumeMounts"
	ContainerFieldWindowsOptions           = "windowsOptions"
	ContainerFieldWorkingDir               = "workingDir"
)

type Container struct {
	AllowPrivilegeEscalation *bool                          `json:"allowPrivilegeEscalation,omitempty" yaml:"allowPrivilegeEscalation,omitempty"`
	AppArmorProfile          *AppArmorProfile               `json:"appArmorProfile,omitempty" yaml:"appArmorProfile,omitempty"`
	CapAdd                   []string                       `json:"capAdd,omitempty" yaml:"capAdd,omitempty"`
	CapDrop                  []string                       `json:"capDrop,omitempty" yaml:"capDrop,omitempty"`
	Command                  []string                       `json:"command,omitempty" yaml:"command,omitempty"`
	Entrypoint               []string                       `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Env                      []EnvVar                       `json:"env,omitempty" yaml:"env,omitempty"`
	EnvFrom                  []EnvFromSource                `json:"envFrom,omitempty" yaml:"envFrom,omitempty"`
	Environment              map[string]string              `json:"environment,omitempty" yaml:"environment,omitempty"`
	EnvironmentFrom          []EnvironmentFrom              `json:"environmentFrom,omitempty" yaml:"environmentFrom,omitempty"`
	ExitCode                 *int64                         `json:"exitCode,omitempty" yaml:"exitCode,omitempty"`
	Image                    string                         `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullPolicy          string                         `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	InitContainer            bool                           `json:"initContainer,omitempty" yaml:"initContainer,omitempty"`
	LivenessProbe            *Probe                         `json:"livenessProbe,omitempty" yaml:"livenessProbe,omitempty"`
	Name                     string                         `json:"name,omitempty" yaml:"name,omitempty"`
	Ports                    []ContainerPort                `json:"ports,omitempty" yaml:"ports,omitempty"`
	PostStart                *LifecycleHandler              `json:"postStart,omitempty" yaml:"postStart,omitempty"`
	PreStop                  *LifecycleHandler              `json:"preStop,omitempty" yaml:"preStop,omitempty"`
	Privileged               *bool                          `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	ProcMount                string                         `json:"procMount,omitempty" yaml:"procMount,omitempty"`
	ReadOnly                 *bool                          `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	ReadinessProbe           *Probe                         `json:"readinessProbe,omitempty" yaml:"readinessProbe,omitempty"`
	ResizePolicy             []ContainerResizePolicy        `json:"resizePolicy,omitempty" yaml:"resizePolicy,omitempty"`
	Resources                *ResourceRequirements          `json:"resources,omitempty" yaml:"resources,omitempty"`
	RestartCount             int64                          `json:"restartCount,omitempty" yaml:"restartCount,omitempty"`
	RestartPolicy            string                         `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup               *int64                         `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot             *bool                          `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	SeccompProfile           *SeccompProfile                `json:"seccompProfile,omitempty" yaml:"seccompProfile,omitempty"`
	StartupProbe             *Probe                         `json:"startupProbe,omitempty" yaml:"startupProbe,omitempty"`
	State                    string                         `json:"state,omitempty" yaml:"state,omitempty"`
	Stdin                    bool                           `json:"stdin,omitempty" yaml:"stdin,omitempty"`
	StdinOnce                bool                           `json:"stdinOnce,omitempty" yaml:"stdinOnce,omitempty"`
	StopSignal               string                         `json:"stopSignal,omitempty" yaml:"stopSignal,omitempty"`
	TTY                      bool                           `json:"tty,omitempty" yaml:"tty,omitempty"`
	TerminationMessagePath   string                         `json:"terminationMessagePath,omitempty" yaml:"terminationMessagePath,omitempty"`
	TerminationMessagePolicy string                         `json:"terminationMessagePolicy,omitempty" yaml:"terminationMessagePolicy,omitempty"`
	Transitioning            string                         `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`
	TransitioningMessage     string                         `json:"transitioningMessage,omitempty" yaml:"transitioningMessage,omitempty"`
	Uid                      *int64                         `json:"uid,omitempty" yaml:"uid,omitempty"`
	VolumeDevices            []VolumeDevice                 `json:"volumeDevices,omitempty" yaml:"volumeDevices,omitempty"`
	VolumeMounts             []VolumeMount                  `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty"`
	WindowsOptions           *WindowsSecurityContextOptions `json:"windowsOptions,omitempty" yaml:"windowsOptions,omitempty"`
	WorkingDir               string                         `json:"workingDir,omitempty" yaml:"workingDir,omitempty"`
}
