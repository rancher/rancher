package client

const (
	DaemonSetConfigType                      = "daemonSetConfig"
	DaemonSetConfigFieldMinReadySeconds      = "minReadySeconds"
	DaemonSetConfigFieldRevisionHistoryLimit = "revisionHistoryLimit"
	DaemonSetConfigFieldUpdateStrategy       = "updateStrategy"
)

type DaemonSetConfig struct {
	MinReadySeconds      *int64                   `json:"minReadySeconds,omitempty"`
	RevisionHistoryLimit *int64                   `json:"revisionHistoryLimit,omitempty"`
	UpdateStrategy       *DaemonSetUpdateStrategy `json:"updateStrategy,omitempty"`
}
