package client

const (
	DaemonSetStatusType                        = "daemonSetStatus"
	DaemonSetStatusFieldCollisionCount         = "collisionCount"
	DaemonSetStatusFieldConditions             = "conditions"
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
	CollisionCount         *int64               `json:"collisionCount,omitempty" yaml:"collisionCount,omitempty"`
	Conditions             []DaemonSetCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	CurrentNumberScheduled *int64               `json:"currentNumberScheduled,omitempty" yaml:"currentNumberScheduled,omitempty"`
	DesiredNumberScheduled *int64               `json:"desiredNumberScheduled,omitempty" yaml:"desiredNumberScheduled,omitempty"`
	NumberAvailable        *int64               `json:"numberAvailable,omitempty" yaml:"numberAvailable,omitempty"`
	NumberMisscheduled     *int64               `json:"numberMisscheduled,omitempty" yaml:"numberMisscheduled,omitempty"`
	NumberReady            *int64               `json:"numberReady,omitempty" yaml:"numberReady,omitempty"`
	NumberUnavailable      *int64               `json:"numberUnavailable,omitempty" yaml:"numberUnavailable,omitempty"`
	ObservedGeneration     *int64               `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	UpdatedNumberScheduled *int64               `json:"updatedNumberScheduled,omitempty" yaml:"updatedNumberScheduled,omitempty"`
}
