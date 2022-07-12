package client

const (
	DrainOptionsType                                 = "drainOptions"
	DrainOptionsFieldDeleteEmptyDirData              = "deleteEmptyDirData"
	DrainOptionsFieldDisableEviction                 = "disableEviction"
	DrainOptionsFieldEnabled                         = "enabled"
	DrainOptionsFieldForce                           = "force"
	DrainOptionsFieldGracePeriod                     = "gracePeriod"
	DrainOptionsFieldIgnoreDaemonSets                = "ignoreDaemonSets"
	DrainOptionsFieldIgnoreErrors                    = "ignoreErrors"
	DrainOptionsFieldPostDrainHooks                  = "postDrainHooks"
	DrainOptionsFieldPreDrainHooks                   = "preDrainHooks"
	DrainOptionsFieldSkipWaitForDeleteTimeoutSeconds = "skipWaitForDeleteTimeoutSeconds"
	DrainOptionsFieldTimeout                         = "timeout"
)

type DrainOptions struct {
	DeleteEmptyDirData              bool        `json:"deleteEmptyDirData,omitempty" yaml:"deleteEmptyDirData,omitempty"`
	DisableEviction                 bool        `json:"disableEviction,omitempty" yaml:"disableEviction,omitempty"`
	Enabled                         bool        `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Force                           bool        `json:"force,omitempty" yaml:"force,omitempty"`
	GracePeriod                     int64       `json:"gracePeriod,omitempty" yaml:"gracePeriod,omitempty"`
	IgnoreDaemonSets                *bool       `json:"ignoreDaemonSets,omitempty" yaml:"ignoreDaemonSets,omitempty"`
	IgnoreErrors                    bool        `json:"ignoreErrors,omitempty" yaml:"ignoreErrors,omitempty"`
	PostDrainHooks                  []DrainHook `json:"postDrainHooks,omitempty" yaml:"postDrainHooks,omitempty"`
	PreDrainHooks                   []DrainHook `json:"preDrainHooks,omitempty" yaml:"preDrainHooks,omitempty"`
	SkipWaitForDeleteTimeoutSeconds int64       `json:"skipWaitForDeleteTimeoutSeconds,omitempty" yaml:"skipWaitForDeleteTimeoutSeconds,omitempty"`
	Timeout                         int64       `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
