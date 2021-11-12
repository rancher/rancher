package client

const (
	CatalogStatusType                      = "catalogStatus"
	CatalogStatusFieldCommit               = "commit"
	CatalogStatusFieldConditions           = "conditions"
	CatalogStatusFieldHelmVersionCommits   = "helmVersionCommits"
	CatalogStatusFieldLastRefreshTimestamp = "lastRefreshTimestamp"
)

type CatalogStatus struct {
	Commit               string                    `json:"commit,omitempty" yaml:"commit,omitempty"`
	Conditions           []CatalogCondition        `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	HelmVersionCommits   map[string]VersionCommits `json:"helmVersionCommits,omitempty" yaml:"helmVersionCommits,omitempty"`
	LastRefreshTimestamp string                    `json:"lastRefreshTimestamp,omitempty" yaml:"lastRefreshTimestamp,omitempty"`
}
