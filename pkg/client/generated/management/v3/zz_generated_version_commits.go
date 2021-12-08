package client

const (
	VersionCommitsType       = "versionCommits"
	VersionCommitsFieldValue = "Value"
)

type VersionCommits struct {
	Value map[string]string `json:"Value,omitempty" yaml:"Value,omitempty"`
}
