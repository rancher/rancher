package client

const (
	CatalogStatusType        = "catalogStatus"
	CatalogStatusFieldCommit = "commit"
)

type CatalogStatus struct {
	Commit string `json:"commit,omitempty"`
}
