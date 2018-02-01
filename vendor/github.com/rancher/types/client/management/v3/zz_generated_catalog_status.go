package client

const (
	CatalogStatusType                      = "catalogStatus"
	CatalogStatusFieldCommit               = "commit"
	CatalogStatusFieldHelmVersionCommits   = "helmVersionCommits"
	CatalogStatusFieldLastRefreshTimestamp = "lastRefreshTimestamp"
)

type CatalogStatus struct {
	Commit               string                    `json:"commit,omitempty"`
	HelmVersionCommits   map[string]VersionCommits `json:"helmVersionCommits,omitempty"`
	LastRefreshTimestamp string                    `json:"lastRefreshTimestamp,omitempty"`
}
