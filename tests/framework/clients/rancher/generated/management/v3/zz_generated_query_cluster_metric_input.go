package client

const (
	QueryClusterMetricInputType             = "queryClusterMetricInput"
	QueryClusterMetricInputFieldClusterName = "clusterId"
	QueryClusterMetricInputFieldExpr        = "expr"
	QueryClusterMetricInputFieldFrom        = "from"
	QueryClusterMetricInputFieldInterval    = "interval"
	QueryClusterMetricInputFieldTo          = "to"
)

type QueryClusterMetricInput struct {
	ClusterName string `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Expr        string `json:"expr,omitempty" yaml:"expr,omitempty"`
	From        string `json:"from,omitempty" yaml:"from,omitempty"`
	Interval    string `json:"interval,omitempty" yaml:"interval,omitempty"`
	To          string `json:"to,omitempty" yaml:"to,omitempty"`
}
