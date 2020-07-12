package client

const (
	NodeDrainInputType                  = "nodeDrainInput"
	NodeDrainInputFieldDeleteLocalData  = "deleteLocalData"
	NodeDrainInputFieldForce            = "force"
	NodeDrainInputFieldGracePeriod      = "gracePeriod"
	NodeDrainInputFieldIgnoreDaemonSets = "ignoreDaemonSets"
	NodeDrainInputFieldTimeout          = "timeout"
)

type NodeDrainInput struct {
	DeleteLocalData  bool  `json:"deleteLocalData,omitempty" yaml:"deleteLocalData,omitempty"`
	Force            bool  `json:"force,omitempty" yaml:"force,omitempty"`
	GracePeriod      int64 `json:"gracePeriod,omitempty" yaml:"gracePeriod,omitempty"`
	IgnoreDaemonSets *bool `json:"ignoreDaemonSets,omitempty" yaml:"ignoreDaemonSets,omitempty"`
	Timeout          int64 `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}
