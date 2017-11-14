package client

const (
	DaemonSetStatusType                        = "daemonSetStatus"
	DaemonSetStatusFieldCollisionCount         = "collisionCount"
	DaemonSetStatusFieldCurrentNumberScheduled = "currentNumberScheduled"
	DaemonSetStatusFieldDesiredNumberScheduled = "desiredNumberScheduled"
	DaemonSetStatusFieldNumberAvailable        = "numberAvailable"
	DaemonSetStatusFieldNumberMisscheduled     = "numberMisscheduled"
	DaemonSetStatusFieldNumberReady            = "numberReady"
	DaemonSetStatusFieldNumberUnavailable      = "numberUnavailable"
	DaemonSetStatusFieldObservedGeneration     = "observedGeneration"
	DaemonSetStatusFieldUpdatedNumberScheduled = "updatedNumberScheduled"
)

type DaemonSetStatus struct {
	CollisionCount         *int64 `json:"collisionCount,omitempty"`
	CurrentNumberScheduled *int64 `json:"currentNumberScheduled,omitempty"`
	DesiredNumberScheduled *int64 `json:"desiredNumberScheduled,omitempty"`
	NumberAvailable        *int64 `json:"numberAvailable,omitempty"`
	NumberMisscheduled     *int64 `json:"numberMisscheduled,omitempty"`
	NumberReady            *int64 `json:"numberReady,omitempty"`
	NumberUnavailable      *int64 `json:"numberUnavailable,omitempty"`
	ObservedGeneration     *int64 `json:"observedGeneration,omitempty"`
	UpdatedNumberScheduled *int64 `json:"updatedNumberScheduled,omitempty"`
}
