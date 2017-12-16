package client

const (
	CatalogStatusType                      = "catalogStatus"
	CatalogStatusFieldCommit               = "commit"
	CatalogStatusFieldLastRefreshTimestamp = "lastRefreshTimestamp"
)

type CatalogStatus struct {
	Commit               string `json:"commit,omitempty"`
	LastRefreshTimestamp string `json:"lastRefreshTimestamp,omitempty"`
}
