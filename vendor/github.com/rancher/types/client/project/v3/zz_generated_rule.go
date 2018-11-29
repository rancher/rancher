package client

import "k8s.io/apimachinery/pkg/util/intstr"

const (
	RuleType             = "rule"
	RuleFieldAlert       = "alert"
	RuleFieldAnnotations = "annotations"
	RuleFieldExpr        = "expr"
	RuleFieldFor         = "for"
	RuleFieldLabels      = "labels"
	RuleFieldRecord      = "record"
)

type Rule struct {
	Alert       string             `json:"alert,omitempty" yaml:"alert,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Expr        intstr.IntOrString `json:"expr,omitempty" yaml:"expr,omitempty"`
	For         string             `json:"for,omitempty" yaml:"for,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Record      string             `json:"record,omitempty" yaml:"record,omitempty"`
}
