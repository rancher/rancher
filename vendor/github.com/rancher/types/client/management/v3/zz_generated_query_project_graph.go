package client

const (
	QueryProjectGraphType           = "queryProjectGraph"
	QueryProjectGraphFieldGraphName = "graphID"
	QueryProjectGraphFieldSeries    = "series"
)

type QueryProjectGraph struct {
	GraphName string   `json:"graphID,omitempty" yaml:"graphID,omitempty"`
	Series    []string `json:"series,omitempty" yaml:"series,omitempty"`
}
