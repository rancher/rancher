package client

const (
	SourceCodeConfigType                        = "sourceCodeConfig"
	SourceCodeConfigFieldBranch                 = "branch"
	SourceCodeConfigFieldBranchCondition        = "branchCondition"
	SourceCodeConfigFieldSourceCodeCredentialId = "sourceCodeCredentialId"
	SourceCodeConfigFieldURL                    = "url"
)

type SourceCodeConfig struct {
	Branch                 string `json:"branch,omitempty"`
	BranchCondition        string `json:"branchCondition,omitempty"`
	SourceCodeCredentialId string `json:"sourceCodeCredentialId,omitempty"`
	URL                    string `json:"url,omitempty"`
}
