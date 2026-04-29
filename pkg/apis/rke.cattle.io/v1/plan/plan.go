package plan

import (
	"hash"

	planapi "github.com/rancher/rancher/pkg/plan"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type OneTimeInstruction = planapi.OneTimeInstruction
type PeriodicInstruction = planapi.PeriodicInstruction
type PeriodicInstructionOutput = planapi.PeriodicInstructionOutput
type ProbeStatus = planapi.ProbeStatus

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
}

type Secret struct {
	ServerToken string `json:"serverToken,omitempty"`
	AgentToken  string `json:"agentToken,omitempty"`
}

type File struct {
	Content     string `json:"content,omitempty"`
	Path        string `json:"path,omitempty"`
	Permissions string `json:"permissions,omitempty"`
	Dynamic     bool   `json:"dynamic,omitempty"`
	// Minor signifies that the file can be changed on a node without having to cause a full-blown drain/cordon operation
	Minor bool `json:"minor,omitempty"`

	// DrainHashFunc specifies a custom hash function to be used by the planner when this file is processed while
	// updating the plans DRAIN_HASH value, as opposed to the default behavior of directly hashing the contents of the
	// File.
	// This allows the planner to better determine when a given File operation should drain or restart a node (or both).
	// This can be useful when changing certain properties of the rke2/k3s ConfigYamlFileName file, such as the "server"
	// arg, which should restart the distribution but not trigger a drain (so long as that was the only change within
	// the File).
	DrainHashFunc func(hash.Hash) error `json:"-"`
}

// NodePlan is the struct used to deliver instructions/files/probes to the system-agent, and retrieve feedback
type NodePlan struct {
	Files                []File                `json:"files,omitempty"`
	Instructions         []OneTimeInstruction  `json:"instructions,omitempty"`
	PeriodicInstructions []PeriodicInstruction `json:"periodicInstructions,omitempty"`
	Error                string                `json:"error,omitempty"`
	Probes               map[string]Probe      `json:"probes,omitempty"`
}
