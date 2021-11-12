package client

const (
	RKEConfigNodePlanType             = "rkeConfigNodePlan"
	RKEConfigNodePlanFieldAddress     = "address"
	RKEConfigNodePlanFieldAnnotations = "annotations"
	RKEConfigNodePlanFieldFiles       = "files"
	RKEConfigNodePlanFieldLabels      = "labels"
	RKEConfigNodePlanFieldPortChecks  = "portChecks"
	RKEConfigNodePlanFieldProcesses   = "processes"
	RKEConfigNodePlanFieldTaints      = "taints"
)

type RKEConfigNodePlan struct {
	Address     string             `json:"address,omitempty" yaml:"address,omitempty"`
	Annotations map[string]string  `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	Files       []File             `json:"files,omitempty" yaml:"files,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty" yaml:"labels,omitempty"`
	PortChecks  []PortCheck        `json:"portChecks,omitempty" yaml:"portChecks,omitempty"`
	Processes   map[string]Process `json:"processes,omitempty" yaml:"processes,omitempty"`
	Taints      []RKETaint         `json:"taints,omitempty" yaml:"taints,omitempty"`
}
