package kstatus

import "github.com/rancher/wrangler/pkg/condition"

// Conditions read by the kstatus package

const (
	Reconciling = condition.Cond("Reconciling")
	Stalled     = condition.Cond("Stalled")
)

func SetError(obj interface{}, message string) {
	Reconciling.False(obj)
	Reconciling.Message(obj, "")
	Reconciling.Reason(obj, "")
	Stalled.True(obj)
	Stalled.Reason(obj, string(Stalled))
	Stalled.Message(obj, message)
}

func SetTransitioning(obj interface{}, message string) {
	Reconciling.True(obj)
	Reconciling.Message(obj, message)
	Reconciling.Reason(obj, string(Reconciling))
	Stalled.False(obj)
	Stalled.Reason(obj, "")
	Stalled.Message(obj, "")
}

func SetActive(obj interface{}) {
	Reconciling.False(obj)
	Reconciling.Message(obj, "")
	Reconciling.Reason(obj, "")
	Stalled.False(obj)
	Stalled.Reason(obj, "")
	Stalled.Message(obj, "")
}
