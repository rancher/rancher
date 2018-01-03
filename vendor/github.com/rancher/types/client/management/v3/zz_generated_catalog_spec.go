package client

const (
	CatalogSpecType             = "catalogSpec"
	CatalogSpecFieldBranch      = "branch"
	CatalogSpecFieldCatalogKind = "catalogKind"
	CatalogSpecFieldDescription = "description"
	CatalogSpecFieldURL         = "url"
)

type CatalogSpec struct {
	Branch      string `json:"branch,omitempty"`
	CatalogKind string `json:"catalogKind,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}
