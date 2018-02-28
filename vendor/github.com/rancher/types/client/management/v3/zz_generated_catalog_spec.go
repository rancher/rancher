package client

const (
	CatalogSpecType             = "catalogSpec"
	CatalogSpecFieldBranch      = "branch"
	CatalogSpecFieldCatalogKind = "catalogKind"
	CatalogSpecFieldDescription = "description"
	CatalogSpecFieldURL         = "url"
)

type CatalogSpec struct {
	Branch      string `json:"branch,omitempty" yaml:"branch,omitempty"`
	CatalogKind string `json:"catalogKind,omitempty" yaml:"catalogKind,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
}
