package client

const (
	MultiClusterAppSpecType                      = "multiClusterAppSpec"
	MultiClusterAppSpecFieldAnswers              = "answers"
	MultiClusterAppSpecFieldMembers              = "members"
	MultiClusterAppSpecFieldRevisionHistoryLimit = "revisionHistoryLimit"
	MultiClusterAppSpecFieldRoles                = "roles"
	MultiClusterAppSpecFieldTargets              = "targets"
	MultiClusterAppSpecFieldTemplateVersionID    = "templateVersionId"
	MultiClusterAppSpecFieldTimeout              = "timeout"
	MultiClusterAppSpecFieldUpgradeStrategy      = "upgradeStrategy"
	MultiClusterAppSpecFieldWait                 = "wait"
)

type MultiClusterAppSpec struct {
	Answers              []Answer         `json:"answers,omitempty" yaml:"answers,omitempty"`
	Members              []Member         `json:"members,omitempty" yaml:"members,omitempty"`
	RevisionHistoryLimit int64            `json:"revisionHistoryLimit,omitempty" yaml:"revisionHistoryLimit,omitempty"`
	Roles                []string         `json:"roles,omitempty" yaml:"roles,omitempty"`
	Targets              []Target         `json:"targets,omitempty" yaml:"targets,omitempty"`
	TemplateVersionID    string           `json:"templateVersionId,omitempty" yaml:"templateVersionId,omitempty"`
	Timeout              int64            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	UpgradeStrategy      *UpgradeStrategy `json:"upgradeStrategy,omitempty" yaml:"upgradeStrategy,omitempty"`
	Wait                 bool             `json:"wait,omitempty" yaml:"wait,omitempty"`
}
