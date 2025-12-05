package plan

import (
	"hash"

	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

type Plan struct {
	Nodes    map[string]*Node         `json:"nodes,omitempty"`
	Machines map[string]*capi.Machine `json:"machines,omitempty"`
	Metadata map[string]*Metadata     `json:"metadata,omitempty"`
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

type PeriodicInstructionOutput struct {
	Name                  string `json:"name"`
	Stdout                []byte `json:"stdout"`                // Stdout is a byte array of the gzip+base64 stdout output
	Stderr                []byte `json:"stderr"`                // Stderr is a byte array of the gzip+base64 stderr output
	ExitCode              int    `json:"exitCode"`              // ExitCode is an int representing the exit code of the last run instruction
	LastSuccessfulRunTime string `json:"lastSuccessfulRunTime"` // LastSuccessfulRunTime is a time.UnixDate formatted string of the last time the instruction was run
}

type Secret struct {
	ServerToken string `json:"serverToken,omitempty"`
	AgentToken  string `json:"agentToken,omitempty"`
}

type OneTimeInstruction struct {
	Name       string   `json:"name,omitempty"`
	Image      string   `json:"image,omitempty"`
	Env        []string `json:"env,omitempty"`
	Args       []string `json:"args,omitempty"`
	Command    string   `json:"command,omitempty"`
	SaveOutput bool     `json:"saveOutput,omitempty"`
}

type PeriodicInstruction struct {
	Name          string   `json:"name,omitempty"`
	Image         string   `json:"image,omitempty"`
	Env           []string `json:"env,omitempty"`
	Args          []string `json:"args,omitempty"`
	Command       string   `json:"command,omitempty"`
	PeriodSeconds int      `json:"periodSeconds,omitempty"` // default 600, i.e. 10 minutes
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
