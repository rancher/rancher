package client

const (
	StatefulSetOrdinalsType       = "statefulSetOrdinals"
	StatefulSetOrdinalsFieldStart = "start"
)

type StatefulSetOrdinals struct {
	Start int64 `json:"start,omitempty" yaml:"start,omitempty"`
}
