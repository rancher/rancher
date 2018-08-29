package jenkins

const (
	CreateJobURI          = "/createItem"
	UpdateJobURI          = "/job/%s/config.xml"
	StopJobURI            = "/job/%s/%d/stop"
	CancelQueueItemURI    = "/queue/cancelItem?id=%d"
	DeleteBuildURI        = "/job/%s/%d/doDelete"
	GetCrumbURI           = "/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)"
	JenkinsJobBuildURI    = "/job/%s/build"
	JenkinsJobInfoURI     = "/job/%s/api/json"
	JenkinsSetCredURI     = "/credentials/store/system/domain/_/createCredentials"
	JenkinsGetCredURI     = "/credentials/store/system/domain/_/credential/%s/api/json"
	JenkinsDeleteCredURI  = "/credentials/store/system/domain/_/credential/%s/doDelete"
	JenkinsBuildInfoURI   = "/job/%s/lastBuild/api/json"
	JenkinsWFBuildInfoURI = "/job/%s/lastBuild/wfapi"
	JenkinsWFNodeInfoURI  = "/job/%s/lastBuild/execution/node/%s/wfapi"
	JenkinsWFNodeLogURI   = "/job/%s/lastBuild/execution/node/%s/wfapi/log"
	JenkinsBuildLogURI    = "/job/%s/%d/timestamps/?elapsed=HH'h'mm'm'ss's'S'ms'&appendLog"
	ScriptURI             = "/scriptText"
	PrepareWFNodeID       = "5"

	markSkipScript = "Utils.markStageSkippedForConditional('%s')"

	WorkflowJobPlugin    = "workflow-job@2.17"
	FlowDefinitionClass  = "org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition"
	FlowDefinitionPlugin = "workflow-cps@2.43"
	JenkinsJobPrefix     = "pipeline_"
)
