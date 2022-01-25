package plan

import capi "sigs.k8s.io/cluster-api/api/v1beta1"

type Plan struct {
	Nodes    map[string]*Node         `json:"nodes,omitempty"`
	Machines map[string]*capi.Machine `json:"machines,omitempty"`
	Metadata map[string]*Metadata     `json:"metadata,omitempty"`
	Cluster  *capi.Cluster            `json:"cluster,omitempty"`
}

type Metadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ProbeStatus struct {
	Healthy      bool `json:"healthy,omitempty"`
	SuccessCount int  `json:"successCount,omitempty"`
	FailureCount int  `json:"failureCount,omitempty"`
}

type Node struct {
	Plan        NodePlan               `json:"plan,omitempty"`
	AppliedPlan *NodePlan              `json:"appliedPlan,omitempty"`
	Output      map[string][]byte      `json:"-"`
	Failed      bool                   `json:"failed,omitempty"`
	InSync      bool                   `json:"inSync,omitempty"`
	Healthy     bool                   `json:"healthy,omitempty"`
	ProbeStatus map[string]ProbeStatus `json:"probeStatus,omitempty"`
}

type Secret struct {
	ServerToken string `json:"serverToken,omitempty"`
	AgentToken  string `json:"agentToken,omitempty"`
}

type Instruction struct {
	Name       string   `json:"name,omitempty"`
	Image      string   `json:"image,omitempty"`
	Env        []string `json:"env,omitempty"`
	Args       []string `json:"args,omitempty"`
	Command    string   `json:"command,omitempty"`
	SaveOutput bool     `json:"saveOutput,omitempty"`
}

type File struct {
	Content string `json:"content,omitempty"`
	Path    string `json:"path,omitempty"`
	Dynamic bool   `json:"dynamic,omitempty"`
}

type NodePlan struct {
	Files        []File           `json:"files,omitempty"`
	Instructions []Instruction    `json:"instructions,omitempty"`
	Error        string           `json:"error,omitempty"`
	Probes       map[string]Probe `json:"probes,omitempty"`
}
