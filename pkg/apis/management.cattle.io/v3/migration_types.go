package v3

import (
	"unicode/utf8"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MigrationSummary describes the overall state of a migration or a single
// migration run.
type MigrationSummary string

const (
	// MigrationSummaryComplete indicates the migration has completed.
	MigrationSummaryComplete MigrationSummary = "Complete"
	// MigrationSummaryInProgress indicates the migration is underway.
	MigrationSummaryInProgress MigrationSummary = "In Progress"
	// MigrationSummaryError indicates an error occurred during the migration.
	MigrationSummaryError MigrationSummary = "Error"
	// MigrationSummaryPending indicates the migration is pending and has not yet started.
	MigrationSummaryPending MigrationSummary = "Pending"
	// MigrationSummaryRunning indicates the migration is currently running.
	MigrationSummaryRunning MigrationSummary = "Running"
	// MigrationSummaryInterrupted indicates the migration was interrupted before completion.
	MigrationSummaryInterrupted MigrationSummary = "Interrupted"
)

// MigrationCondition is the condition types for a migration.
type MigrationCondition string

const (
	// MigrationFailed indicates the migration has failed.
	MigrationFailed MigrationCondition = "Failed"
	// MigrationComplete indicates the migration has completed successfully.
	MigrationComplete MigrationCondition = "Complete"
)

const (
	// MaxMigrationRuns is the maximum number of migration runs retained in the
	// status history. Older runs beyond this limit should be pruned.
	MaxMigrationRuns = 20
	// MaxMigrationErrors is the maximum number of errors retained for a single
	// migration run.
	MaxMigrationErrors = 100
)

const (
	// MaxMigrationErrorTypeLength bounds MigrationError.Type.
	MaxMigrationErrorTypeLength = 128
	// MaxMigrationErrorReasonLength bounds MigrationError.Reason.
	MaxMigrationErrorReasonLength = 512
	// MaxMigrationErrorMessageLength bounds MigrationError.Message.
	MaxMigrationErrorMessageLength = 1024
)

// Truncate clamps the error's fields to the lengths allowed by the CRD schema.
// The API server rejects writes that exceed them, so callers must call this
// before recording an error.
func (e *MigrationError) Truncate() {
	e.Type = truncateRunes(e.Type, MaxMigrationErrorTypeLength)
	e.Reason = truncateRunes(e.Reason, MaxMigrationErrorReasonLength)
	e.Message = truncateRunes(e.Message, MaxMigrationErrorMessageLength)
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	const ellipsis = "…"
	return string([]rune(s)[:max-1]) + ellipsis
}

// AppendError records err against the run. MigrationsFailed is always
// incremented; Errors retains at most MaxMigrationErrors entries, keeping the
// earliest failures since later ones are usually cascades.
func (r *MigrationRun) AppendError(e MigrationError) {
	r.MigrationsFailed++
	if len(r.Errors) >= MaxMigrationErrors {
		return
	}
	e.Truncate()
	r.Errors = append(r.Errors, e)
}

// PrependRun records a completed run, keeping MigrationRuns ordered most
// recent first and bounded to MaxMigrationRuns.
func (s *MigrationStatus) PrependRun(run MigrationRun) {
	s.MigrationRuns = append([]MigrationRun{run}, s.MigrationRuns...)
	if len(s.MigrationRuns) > MaxMigrationRuns {
		s.MigrationRuns = s.MigrationRuns[:MaxMigrationRuns]
	}
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Summary",type="string",JSONPath=".status.summary"
// +kubebuilder:printcolumn:name="Resources Migrated",type="integer",JSONPath=".status.totalResourcesMigrated"

// Migration is the source of truth for a single Rancher custom resource
// migration. It tracks the configuration of the migration, a history of every
// time the migration was run, and the current status of the migration.
type Migration struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the desired configuration for the migration.
	// +optional
	Spec MigrationSpec `json:"spec,omitempty"`

	// Status is the most recently observed status of the migration.
	// +optional
	Status MigrationStatus `json:"status,omitempty"`
}

// MigrationSpec is the specification of a Migration.
type MigrationSpec struct {
	// Description is a human readable description of what is being migrated and why.
	// +optional
	Description string `json:"description,omitempty"`

	// VersionAffected is the version of Rancher impacted by this migration. It can
	// be a range (for example ">= 2.11.4 < 2.13.0").
	// +optional
	VersionAffected string `json:"versionAffected,omitempty"`

	// ManualRunRequestID is a mechanism to trigger a manual run of the migration.
	// It is used when auto-apply is disabled, or when a migration needs to be
	// re-run (for example after an error or after it has already completed).
	//
	// To request a run, set this to a new, unique value (any string that differs
	// from the previous value, such as a timestamp or UUID). The controller runs
	// the migration once whenever this value differs from
	// status.observedRunRequestID, then records the handled value in status so the
	// same request is not processed again. Re-applying the same value is a no-op.
	// +optional
	ManualRunRequestID string `json:"manualRunRequestID,omitempty"`

	// CVEFixNumbers optionally lists the CVE numbers that this migration fixes.
	// +optional
	CVEFixNumbers []string `json:"cveFixNumbers,omitempty"`

	// BatchSize optionally overrides the global migration-default-batch-size
	// setting for this migration. It controls how many migrations are attempted
	// before pausing.
	// +optional
	// +kubebuilder:validation:Minimum=1
	BatchSize *int `json:"batchSize,omitempty"`

	// BatchDelay optionally overrides the global migration-default-batch-delay
	// setting for this migration. It controls how long, in seconds, to pause
	// between batches.
	// +optional
	// +kubebuilder:validation:Minimum=0
	BatchDelay *int `json:"batchDelay,omitempty"`
}

// MigrationRun records a single execution of a migration.
type MigrationRun struct {
	// MigrationStartTime is when this specific migration run was started.
	// +optional
	MigrationStartTime metav1.Time `json:"migrationStartTime,omitempty"`

	// MigrationEndTime is when this migration run was completed.
	// +optional
	MigrationEndTime metav1.Time `json:"migrationEndTime,omitempty"`

	// Summary is the end result of the migration run.
	// +optional
	// +kubebuilder:validation:Enum=Complete;"In Progress";Error;"Not Run"
	Summary MigrationSummary `json:"summary,omitempty"`

	// MigrationsPerformedSuccessfully is the number of migrations successfully
	// completed during this run.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MigrationsPerformedSuccessfully int `json:"migrationsPerformedSuccessfully"`

	// MigrationsFailed is the number of migrations that failed during this run.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MigrationsFailed int `json:"migrationsFailed"`

	// Version is the version of Rancher in which the migration was run.
	// +optional
	Version string `json:"version,omitempty"`

	// Errors is a list of all errors that occurred during this migration run.
	// +optional
	// +kubebuilder:validation:MaxItems=100
	Errors []MigrationError `json:"errors,omitempty"`
}

// MigrationError describes a single error that occurred during a migration run.
type MigrationError struct {
	// Type is a programmatic identifier for the error, in CamelCase.
	// +optional
	// +kubebuilder:validation:MaxLength=128
	// +kubebuilder:validation:Pattern=`^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$`
	Type string `json:"type,omitempty"`

	// Reason is a short human readable explanation of the issue.
	// +optional
	// +kubebuilder:validation:MaxLength=512
	Reason string `json:"reason,omitempty"`

	// Message is the error message, truncated to fit. Developers must not
	// include sensitive data such as secrets, tokens, or passwords.
	// +optional
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message,omitempty"`
}

// MigrationStatus is the most recently observed status of a Migration.
type MigrationStatus struct {
	// LastUpdateTime is when the status was last updated.
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// Summary is the state of the whole migration.
	// +optional
	// +kubebuilder:validation:Enum=Complete;"In Progress";Error;"Not Run"
	Summary MigrationSummary `json:"summary,omitempty"`

	// TotalResourcesMigrated is the number of resources that have been migrated.
	// It should be the sum of all migrationHistories.migrationsPerformedSuccessfully.
	// +optional
	// +kubebuilder:validation:Minimum=0
	TotalResourcesMigrated int `json:"totalResourcesMigrated"`

	// MigrationsRequired is a best estimate count of the number of migrations
	// that need to be performed, as determined by the Getter. The migration is
	// considered complete when this number is 0.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MigrationsRequired int `json:"migrationsRequired"`

	// ObservedGeneration is the most recent generation observed by the
	// controller. It corresponds to the CRMigration's metadata.generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ObservedRunRequestID is the most recent spec.manualRunRequestID handled by
	// the controller. When it differs from spec.manualRunRequestID, a manual run
	// has been requested and not yet processed.
	// +optional
	ObservedRunRequestID string `json:"observedRunRequestID,omitempty"`

	// Conditions represent the latest available observations of the migration's
	// state. Use MigrationCondition as the type for the condition.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// MigrationRuns is a history of each time this migration was run. Entries
	// must be ordered from most recent to oldest.
	// +optional
	// +kubebuilder:validation:MaxItems=20
	MigrationRuns []MigrationRun `json:"migrationRuns,omitempty"`
}
