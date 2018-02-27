package client

const (
	CatalogStatusType                      = "catalogStatus"
	CatalogStatusFieldCommit               = "commit"
	CatalogStatusFieldConditions           = "conditions"
	CatalogStatusFieldHelmVersionCommits   = "helmVersionCommits"
	CatalogStatusFieldLastRefreshTimestamp = "lastRefreshTimestamp"
)

type CatalogStatus struct {
	Commit               string                    `json:"commit,omitempty"`
	Conditions           []CatalogCondition        `json:"conditions,omitempty"`
	HelmVersionCommits   map[string]VersionCommits `json:"helmVersionCommits,omitempty"`
	LastRefreshTimestamp string                    `json:"lastRefreshTimestamp,omitempty"`
}
