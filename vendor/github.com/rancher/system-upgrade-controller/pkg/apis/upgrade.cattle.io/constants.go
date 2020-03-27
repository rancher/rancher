package upgrade

import "path"

const (
	// AnnotationTTLSecondsAfterFinished is used to store a fallback value for job.spec.ttlSecondsAfterFinished
	AnnotationTTLSecondsAfterFinished = GroupName + `/ttl-seconds-after-finished`

	// LabelController is the name of the upgrade controller.
	LabelController = GroupName + `/controller`

	// LabelNode is the node being upgraded.
	LabelNode = GroupName + `/node`

	// LabelPlan is the plan being applied.
	LabelPlan = GroupName + `/plan`

	// LabelVersion is the version of the plan being applied.
	LabelVersion = GroupName + `/version`

	// LabelPlanSuffix is used for composing labels specific to a plan.
	LabelPlanSuffix = `plan.` + GroupName
)

func LabelPlanName(name string) string {
	return path.Join(LabelPlanSuffix, name)
}
