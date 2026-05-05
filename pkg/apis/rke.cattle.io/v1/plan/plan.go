package plan

import (
	planapi "github.com/rancher/rancher/pkg/plan"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type OneTimeInstruction = planapi.OneTimeInstruction
type PeriodicInstruction = planapi.PeriodicInstruction
type PeriodicInstructionOutput = planapi.PeriodicInstructionOutput
type ProbeStatus = planapi.ProbeStatus
type File = planapi.File

type Plan struct {
	Nodes    map[string]*Node         `json:"nodes,omitempty"`
	Machines map[string]*capi.Machine `json:"machines,omitempty"`
	Metadata map[string]*Metadata     `json:"metadata,omitempty"`
}

type Metadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type Node struct {
	Plan           NodePlan                             `json:"plan,omitempty"`
	AppliedPlan    *NodePlan                            `json:"appliedPlan,omitempty"`
	JoinedTo       string                               `json:"joinedTo,omitempty"`
	Output         map[string][]byte                    `json:"-"`
	PeriodicOutput map[string]PeriodicInstructionOutput `json:"-"`
	Failed         bool                                 `json:"failed,omitempty"`
	InSync         bool                                 `json:"inSync,omitempty"`
	Healthy        bool                                 `json:"healthy,omitempty"`
	PlanDataExists bool                                 `json:"planDataExists,omitempty"`
	ProbeStatus    map[string]ProbeStatus               `json:"probeStatus,omitempty"`
	ProbesUsable   bool                                 `json:"probesUsable,omitempty"` // ProbesUsable indicates that the probes have passed at least once for the appliedPlan
	PlanState      planapi.PlanState                    `json:"planState,omitempty"`
	PlanRevision   int                                  `json:"planRevision,omitempty"`
}

type Secret struct {
	ServerToken string `json:"serverToken,omitempty"`
	AgentToken  string `json:"agentToken,omitempty"`
}

// NodePlan is the struct used to deliver instructions/files/probes to the system-agent, and retrieve feedback
type NodePlan struct {
	Files                []File                `json:"files,omitempty"`
	Instructions         []OneTimeInstruction  `json:"instructions,omitempty"`
	PeriodicInstructions []PeriodicInstruction `json:"periodicInstructions,omitempty"`
	Error                string                `json:"error,omitempty"`
	Probes               map[string]Probe      `json:"probes,omitempty"`
}
