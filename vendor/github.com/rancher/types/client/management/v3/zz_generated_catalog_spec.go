package client

const (
	CatalogSpecType             = "catalogSpec"
	CatalogSpecFieldBranch      = "branch"
	CatalogSpecFieldCatalogKind = "catalogKind"
	CatalogSpecFieldURL         = "url"
)

type CatalogSpec struct {
	Branch      string `json:"branch,omitempty"`
	CatalogKind string `json:"catalogKind,omitempty"`
	URL         string `json:"url,omitempty"`
}
