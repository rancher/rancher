package client

const (
	SourceCodeConfigType                        = "sourceCodeConfig"
	SourceCodeConfigFieldBranch                 = "branch"
	SourceCodeConfigFieldBranchCondition        = "branchCondition"
	SourceCodeConfigFieldSourceCodeCredentialId = "sourceCodeCredentialId"
	SourceCodeConfigFieldURL                    = "url"
)

type SourceCodeConfig struct {
	Branch                 string `json:"branch,omitempty" yaml:"branch,omitempty"`
	BranchCondition        string `json:"branchCondition,omitempty" yaml:"branchCondition,omitempty"`
	SourceCodeCredentialId string `json:"sourceCodeCredentialId,omitempty" yaml:"sourceCodeCredentialId,omitempty"`
	URL                    string `json:"url,omitempty" yaml:"url,omitempty"`
}
