package jenkins

import "encoding/xml"

type PipelineStep struct {
	command          string
	image            string
	containerOptions string
}

type PipelineJob struct {
	XMLName    xml.Name   `xml:"flow-definition"`
	Plugin     string     `xml:"plugin,attr"`
	Definition Definition `xml:"definition"`
}

type Definition struct {
	Class   string `xml:"class,attr"`
	Plugin  string `xml:"plugin,attr"`
	Script  string `xml:"script"`
	Sandbox bool   `xml:"sandbox"`
}

type Project struct {
	XMLName                          xml.Name `xml:"project"`
	Actions                          string   `xml:"actions"`
	Description                      string   `xml:"description"`
	KeepDependencies                 bool     `xml:"keepDependencies"`
	Properties                       string
	Scm                              Scm                     `xml:"scm"`
	AssignedNode                     string                  `xml:"assignedNode"`
	CanRoam                          bool                    `xml:"canRoam"`
	Disabled                         bool                    `xml:"disabled"`
	BlockBuildWhenDownstreamBuilding bool                    `xml:"blockBuildWhenDownstreamBuilding"`
	BlockBuildWhenUpstreamBuilding   bool                    `xml:"blockBuildWhenUpstreamBuilding"`
	Triggers                         Trigger                 `xml:"triggers"`
	ConcurrentBuild                  bool                    `xml:"concurrentBuild"`
	CustomWorkspace                  string                  `xml:"customWorkspace"`
	Builders                         Builder                 `xml:"builders,omitempty"`
	Publishers                       PostBuildTask           `xml:"publishers>org.jvnet.hudson.plugins.groovypostbuild.GroovyPostbuildRecorder"`
	TimeStampWrapper                 TimestampWrapperPlugin  `xml:"buildWrappers>hudson.plugins.timestamper.TimestamperBuildWrapper"`
	TimeoutWrapper                   *TimeoutWrapperPlugin   `xml:"buildWrappers>hudson.plugins.build__timeout.BuildTimeoutWrapper"`
	PreSCMBuildStepsWrapper          PreSCMBuildStepsWrapper `xml:"buildWrappers>org.jenkinsci.plugins.preSCMbuildstep.PreSCMBuildStepsWrapper"`
}

type Scm struct {
	Class                             string `xml:"class,attr"`
	Plugin                            string `xml:"plugin,attr"`
	ConfigVersion                     int    `xml:"configVersion"`
	GitRepo                           string `xml:"userRemoteConfigs>hudson.plugins.git.UserRemoteConfig>url"`
	GitCredentialID                   string `xml:"userRemoteConfigs>hudson.plugins.git.UserRemoteConfig>credentialsId"`
	GitBranch                         string `xml:"branches>hudson.plugins.git.BranchSpec>name"`
	DoGenerateSubmoduleConfigurations bool   `xml:"doGenerateSubmoduleConfigurations"`
	SubmodelCfg                       string `xml:"submoduleCfg,omitempty"`
	Extensions                        string `xml:"extensions"`
}

type Trigger struct {
	BuildTrigger             *BuildTrigger `xml:"jenkins.triggers.ReverseBuildTrigger,omitempty"`
	FanInReverseBuildTrigger *BuildTrigger `xml:"org.lonkar.jobfanin.FanInReverseBuildTrigger,omitempty"`
}

type BuildTrigger struct {
	Spec                     string `xml:"spec"`
	Plugin                   string `xml:"plugin,attr"`
	UpstreamProjects         string `xml:"upstreamProjects"`
	UpsteamProjects          string `xml:"upsteamProjects"`
	WatchUpstreamRecursively bool   `xml:"watchUpstreamRecursively"`
	ThresholdName            string `xml:"threshold>name"`
	ThresholdOrdinal         int    `xml:"threshold>ordinal"`
	ThresholdColor           string `xml:"threshold>color"`
	ThresholdCompleteBuild   bool   `xml:"threshold>completeBuild"`
}

type TimestampWrapperPlugin struct {
	Plugin string `xml:"plugin,attr"`
}

type TimeoutWrapperPlugin struct {
	Plugin    string          `xml:"plugin,attr"`
	Strategy  TimeoutStrategy `xml:"strategy"`
	Operation string          `xml:"operationList>hudson.plugins.build__timeout.operations.FailOperation"`
}

type TimeoutStrategy struct {
	Class          string `xml:"class,attr"`
	TimeoutMinutes int    `xml:"timeoutMinutes"`
}

type PreSCMBuildStepsWrapper struct {
	Plugin      string `xml:"plugin,attr"`
	FailOnError bool   `xml:"failOnError"`
	Command     string `xml:"buildSteps>hudson.tasks.Shell>command"`
}
type PostBuildTask struct {
	Plugin             string       `xml:"plugin,attr"`
	GroovyScript       GroovyScript `xml:"script"`
	Behavior           int          `xml:"behavior"`
	RunForMatrixParent bool         `xml:"runForMatrixParent"`
}

type GroovyScript struct {
	Plugin  string `xml:"plugin,attr"`
	Script  string `xml:"script"`
	Sandbox bool   `xml:"sandbox"`
}

type Builder struct {
	TaskShells []TaskShell `xml:"hudson.tasks.Shell"`
}

type TaskShell struct {
	Command string `xml:"command"`
}

type Build struct {
	ID                string    `json:"id,omitempty"`
	KeepLog           bool      `json:"keepLog,omitempty"`
	Number            int       `json:"number,omitempty"`
	QueueID           int       `json:"queueId,omitempty"`
	Result            string    `json:"result,omitempty"`
	TimeStamp         int64     `json:"timestamp,omitempty"`
	BuiltOn           string    `json:"builtOn,omitempty"`
	ChangeSet         ChangeSet `json:"chanSet,omitempty"`
	Duration          int       `json:"duration,omitempty"`
	EstimatedDuration int       `json:"estimatedDuration,omitempty"`
	Building          bool      `json:"building,omitempty"`
}

type ChangeSet struct {
	Kind  string
	Items []interface{}
}

type JobInfo struct {
	Class   string `json:"_class"`
	Actions []struct {
		Class string `json:"_class"`
	} `json:"actions"`
	Buildable bool `json:"buildable"`
	Builds    []struct {
		Class  string `json:"_class"`
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"builds"`
	Color              string      `json:"color"`
	ConcurrentBuild    bool        `json:"concurrentBuild"`
	Description        string      `json:"description"`
	DisplayName        string      `json:"displayName"`
	DisplayNameOrNull  interface{} `json:"displayNameOrNull"`
	DownstreamProjects []struct {
		Class string `json:"_class"`
		Color string `json:"color"`
		Name  string `json:"name"`
		URL   string `json:"url"`
	} `json:"downstreamProjects"`
	FirstBuild struct {
		Class  string `json:"_class"`
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"firstBuild"`
	FullDisplayName string `json:"fullDisplayName"`
	FullName        string `json:"fullName"`
	HealthReport    []struct {
		Description   string `json:"description"`
		IconClassName string `json:"iconClassName"`
		IconURL       string `json:"iconUrl"`
		Score         int64  `json:"score"`
	} `json:"healthReport"`
	InQueue          bool `json:"inQueue"`
	KeepDependencies bool `json:"keepDependencies"`
	LastBuild        struct {
		Class  string `json:"_class"`
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"lastBuild"`
	LastCompletedBuild struct {
		Class  string `json:"_class"`
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"lastCompletedBuild"`
	LastFailedBuild struct {
		Class  string `json:"_class"`
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"lastFailedBuild"`
	LastStableBuild       interface{} `json:"lastStableBuild"`
	LastSuccessfulBuild   interface{} `json:"lastSuccessfulBuild"`
	LastUnstableBuild     interface{} `json:"lastUnstableBuild"`
	LastUnsuccessfulBuild struct {
		Class  string `json:"_class"`
		Number int    `json:"number"`
		URL    string `json:"url"`
	} `json:"lastUnsuccessfulBuild"`
	Name            string        `json:"name"`
	NextBuildNumber int           `json:"nextBuildNumber"`
	Property        []interface{} `json:"property"`
	QueueItem       interface{}   `json:"queueItem"`
	Scm             struct {
		Class string `json:"_class"`
	} `json:"scm"`
	UpstreamProjects []interface{} `json:"upstreamProjects"`
	URL              string        `json:"url"`
}

type BuildInfo struct {
	Class   string `json:"_class"`
	Actions []struct {
		Class              string `json:"_class"`
		BuildsByBranchName struct {
			OriginMaster struct {
				Class       string      `json:"_class"`
				BuildNumber int         `json:"buildNumber"`
				BuildResult interface{} `json:"buildResult"`
				Marked      struct {
					SHA1   string `json:"SHA1"`
					Branch []struct {
						SHA1 string `json:"SHA1"`
						Name string `json:"name"`
					} `json:"branch"`
				} `json:"marked"`
				Revision struct {
					SHA1   string `json:"SHA1"`
					Branch []struct {
						SHA1 string `json:"SHA1"`
						Name string `json:"name"`
					} `json:"branch"`
				} `json:"revision"`
			} `json:"origin/master"`
		} `json:"buildsByBranchName"`
		Causes []struct {
			Class            string `json:"_class"`
			ShortDescription string `json:"shortDescription"`
			UserID           string `json:"userId"`
			UserName         string `json:"userName"`
		} `json:"causes"`
		LastBuiltRevision struct {
			SHA1   string `json:"SHA1"`
			Branch []struct {
				SHA1 string `json:"SHA1"`
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"lastBuiltRevision"`
		RemoteUrls []string `json:"remoteUrls"`
		ScmName    string   `json:"scmName"`
	} `json:"actions"`
	Artifacts []interface{} `json:"artifacts"`
	Building  bool          `json:"building"`
	BuiltOn   string        `json:"builtOn"`
	ChangeSet struct {
		Class string        `json:"_class"`
		Items []interface{} `json:"items"`
		Kind  string        `json:"kind"`
	} `json:"changeSet"`
	Description       interface{} `json:"description"`
	DisplayName       string      `json:"displayName"`
	Duration          int64       `json:"duration"`
	EstimatedDuration int64       `json:"estimatedDuration"`
	Executor          interface{} `json:"executor"`
	FullDisplayName   string      `json:"fullDisplayName"`
	ID                string      `json:"id"`
	KeepLog           bool        `json:"keepLog"`
	Number            int         `json:"number"`
	QueueID           int         `json:"queueId"`
	Result            string      `json:"result"`
	Timestamp         int64       `json:"timestamp"`
	URL               string      `json:"url"`
}

type Credential struct {
	Scope       string `json:"scope"`
	ID          string `json:"id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Description string `json:"description"`
	Class       string `json:"$class"`
}

type WFBuildInfo struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Status              string  `json:"status"`
	StartTimeMillis     int64   `json:"startTimeMillis"`
	EndTimeMillis       int64   `json:"endTimeMillis"`
	DurationMillis      int64   `json:"durationMillis"`
	QueueDurationMillis int64   `json:"queueDurationMillis"`
	PauseDurationMillis int64   `json:"pauseDurationMillis"`
	Stages              []Stage `json:"stages"`
}

type Stage struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	ExecNode        string    `json:"execNode"`
	Status          string    `json:"status"`
	StartTimeMillis int64     `json:"startTimeMillis"`
	EndTimeMillis   int64     `json:"endTimeMillis"`
	DurationMillis  int64     `json:"durationMillis"`
	Error           NodeError `json:"error"`
}

type WFNodeInfo struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Status              string  `json:"status"`
	StartTimeMillis     int64   `json:"startTimeMillis"`
	EndTimeMillis       int64   `json:"endTimeMillis"`
	DurationMillis      int64   `json:"durationMillis"`
	ParseDurationMillis int64   `json:"parseDurationMillis"`
	PauseDurationMillis int64   `json:"pauseDurationMillis"`
	StageFlowNodes      []Stage `json:"stageFlowNodes"`
}

type WFNodeLog struct {
	NodeID     string `json:"nodeId"`
	NodeStatus string `json:"nodeStatus"`
	Length     int    `json:"length"`
	HasMore    bool   `json:"hasMore"`
	Text       string `json:"text"`
	ConsoleURL string `json:"consoleUrl"`
}

type FlowNode struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	ExecNode             string    `json:"execNode"`
	Status               string    `json:"status"`
	ParameterDescription string    `json:"parameterDescription"`
	StartTimeMillis      int64     `json:"startTimeMillis"`
	DurationMillis       int64     `json:"durationMillis"`
	PauseDurationMillis  int64     `json:"pauseDurationMillis"`
	Error                NodeError `json:"error"`
}

type NodeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
