package plan

import "hash"

// Plan represents the basic unit of work performed by the system-agent.
type Plan struct {
	Files                []File                `json:"files,omitempty"`
	OneTimeInstructions  []OneTimeInstruction  `json:"instructions,omitempty"`
	Probes               map[string]Probe      `json:"probes,omitempty"`
	PeriodicInstructions []PeriodicInstruction `json:"periodicInstructions,omitempty"`
}

// File represents a file to be written on the node by the system-agent.
// Path is the absolute path on the node (e.g. /etc/kubernetes/ssl/ca.pem).
// Content is base64-encoded. If Directory is true, a directory is created, not a file.
type File struct {
	Content       string                `json:"content,omitempty"`
	Directory     bool                  `json:"directory,omitempty"`
	UID           int                   `json:"uid,omitempty"`
	GID           int                   `json:"gid,omitempty"`
	Path          string                `json:"path,omitempty"`
	Permissions   string                `json:"permissions,omitempty"` // internally, the string will be converted to a uint32 to satisfy os.FileMode
	Action        string                `json:"action,omitempty"`
	Dynamic       bool                  `json:"dynamic,omitempty"`
	Minor         bool                  `json:"minor,omitempty"`
	DrainHashFunc func(hash.Hash) error `json:"-"`
}

// CommonInstruction holds fields shared by all instruction types.
type CommonInstruction struct {
	Name    string   `json:"name,omitempty"`
	Image   string   `json:"image,omitempty"`
	Env     []string `json:"env,omitempty"`
	Args    []string `json:"args,omitempty"`
	Command string   `json:"command,omitempty"`
}

// OneTimeInstruction is an instruction that is executed exactly once.
type OneTimeInstruction struct {
	CommonInstruction
	SaveOutput bool `json:"saveOutput,omitempty"`
}

// PeriodicInstruction is an instruction that is executed on a recurring schedule.
type PeriodicInstruction struct {
	CommonInstruction
	PeriodSeconds    int  `json:"periodSeconds,omitempty"` // default 600, i.e. 10 minutes
	SaveStderrOutput bool `json:"saveStderrOutput,omitempty"`
}

// PeriodicInstructionOutput holds the result of a periodic instruction execution.
// The Stdout and Stderr fields are gzip+base64 encoded byte slices.
type PeriodicInstructionOutput struct {
	Name     string `json:"name"`
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
	ExitCode int    `json:"exitCode"`
	// LastSuccessfulRunTime is a time.UnixDate formatted string of the last successful run.
	LastSuccessfulRunTime string `json:"lastSuccessfulRunTime"`
	// Failures is the number of consecutive times this instruction has failed.
	Failures int `json:"failures"`
	// LastFailedRunTime is a time.UnixDate formatted string of when the instruction started failing.
	LastFailedRunTime string `json:"lastFailedRunTime"`
}
