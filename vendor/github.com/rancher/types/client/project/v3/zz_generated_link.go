package client

const (
	LinkType       = "link"
	LinkFieldAlias = "alias"
	LinkFieldName  = "name"
)

type Link struct {
	Alias string `json:"alias,omitempty"`
	Name  string `json:"name,omitempty"`
}
