package client

const (
	PublishCatalogConfigType                 = "publishCatalogConfig"
	PublishCatalogConfigFieldCatalogTemplate = "catalogTemplate"
	PublishCatalogConfigFieldGitAuthor       = "gitAuthor"
	PublishCatalogConfigFieldGitBranch       = "gitBranch"
	PublishCatalogConfigFieldGitEmail        = "gitEmail"
	PublishCatalogConfigFieldGitURL          = "gitUrl"
	PublishCatalogConfigFieldPath            = "path"
	PublishCatalogConfigFieldVersion         = "version"
)

type PublishCatalogConfig struct {
	CatalogTemplate string `json:"catalogTemplate,omitempty" yaml:"catalogTemplate,omitempty"`
	GitAuthor       string `json:"gitAuthor,omitempty" yaml:"gitAuthor,omitempty"`
	GitBranch       string `json:"gitBranch,omitempty" yaml:"gitBranch,omitempty"`
	GitEmail        string `json:"gitEmail,omitempty" yaml:"gitEmail,omitempty"`
	GitURL          string `json:"gitUrl,omitempty" yaml:"gitUrl,omitempty"`
	Path            string `json:"path,omitempty" yaml:"path,omitempty"`
	Version         string `json:"version,omitempty" yaml:"version,omitempty"`
}
