package client

const (
	CatalogRefreshType          = "catalogRefresh"
	CatalogRefreshFieldCatalogs = "catalogs"
)

type CatalogRefresh struct {
	Catalogs []string `json:"catalogs,omitempty" yaml:"catalogs,omitempty"`
}
