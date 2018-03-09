package utils

const (
	PipelineNamespace = "cattle-pipeline"
	DefaultRegistry   = "index.docker.io"
	DefaultTag        = "latest"

	StepTypeSourceCode   = "sourceCode"
	StepTypeRunScript    = "runScript"
	StepTypePublishImage = "publishImage"
	TriggerTypeCron      = "cron"
	TriggerTypeUser      = "user"
	TriggerTypeWebhook   = "webhook"

	StateWaiting  = "Waiting"
	StateBuilding = "Building"
	StateSuccess  = "Success"
	StateFail     = "Fail"
	StateError    = "Error"
	StateSkip     = "Skipped"
	StateAbort    = "Abort"
	StatePending  = "Pending"
	StateDenied   = "Denied"

	PipelineFinishLabel = "pipeline.management.cattle.io/finish"
	PipelineCronLabel   = "pipeline.management.cattle.io/cron"
)

var PreservedEnvVars = []string{
	"CICD_GIT_COMMIT",
	"CICD_GIT_REPO_NAME",
	"CICD_GIT_BRANCH",
	"CICD_GIT_URL",
	"CICD_PIPELINE_ID",
	"CICD_PIPELINE_NAME",
	"CICD_TRIGGER_TYPE",
	"CICD_EXECUTION_ID",
	"CICD_EXECUTION_SEQUENCE",
}
