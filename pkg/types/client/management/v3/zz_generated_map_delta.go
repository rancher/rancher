package client

const (
	MapDeltaType        = "mapDelta"
	MapDeltaFieldAdd    = "add"
	MapDeltaFieldDelete = "delete"
)

type MapDelta struct {
	Add    map[string]string `json:"add,omitempty" yaml:"add,omitempty"`
	Delete map[string]bool   `json:"delete,omitempty" yaml:"delete,omitempty"`
}
