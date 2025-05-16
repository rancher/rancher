package migrations

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/migrations/changes"
)

const (
	migrationsCMName    string = "rancher-migrations"
	migrationsNamespace        = "cattle-system"
)

// registry for known migrations
// TODO: This could be a map or a struct with a sync.Mutex?
var knownMigrations []Migration

// For passing in additional information to the migration.
type MigrationOptions struct {
	// Continue is a token returned by the migration that allows for batching of
	// changes. The token is opaque and used only by the migration.
	Continue string

	// Limit tells the server to limit the number of returned changes.
	//
	// The migration should only consider this number of resources for
	// determining the limit on changes. For example, Limit: 1 might result in
	// two changes for a single resource.
	Limit int64
}

// ChangeSet is a set of ResourceChanges that are considered to be an
// error-boundary when applying changes.
//
// An error applying a ResourceChange from a ChangeSet indicates that the
// ChangeSet should fail, but that the application process should continue with
// the next ChangeSet.
//
// A migration is represented as a set of ChangeSets.
type ChangeSet []changes.ResourceChange

// MigrationChanges represents the calculated changes to apply to the cluster.
type MigrationChanges struct {
	Continue string
	Changes  []ChangeSet
}

// Migration implementations can be registered with the system.
type Migration interface {
	Name() string

	// Changes should return the set of changes that this migration wants to
	// apply to the cluster.
	Changes(ctx context.Context, client changes.Interface, opts MigrationOptions) (*MigrationChanges, error)
}

// Register registers a migration with the migration mechanism.
func Register(migration Migration) {
	// TODO: Serialize this
	knownMigrations = append(knownMigrations, migration)
}

// Migration holds information about a change applied to a cluster.
type MigrationInfo struct {
	// Name is the registered name of the Migration.
	Name string

	// Applied is true if the Migration has been applied to the connected
	// cluster.
	Applied bool

	// Errors records the error messages generated during apply.
	Errors string
}

// MigrationStatus records the state of a Migration.
type MigrationStatus struct {
	AppliedAt time.Time             `json:"appliedAt"`
	Metrics   *changes.ApplyMetrics `json:"metrics,omitempty"`
	Errors    string                `json:"errors"`
}

// MigrationStatusGetter implementations get the status of a named Migration.
type MigrationStatusGetter interface {
	StatusFor(ctx context.Context, name string) (*MigrationStatus, error)
}

// List lists the migrations available in the system.
func List(ctx context.Context, migrationStatus MigrationStatusGetter) ([]*MigrationInfo, error) {
	var result []*MigrationInfo
	for i := range knownMigrations {
		info, err := statusForMigration(ctx, knownMigrations[i].Name(), migrationStatus)
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}

	return result, nil
}

func statusForMigration(ctx context.Context, name string, migrationStatus MigrationStatusGetter) (*MigrationInfo, error) {
	status, err := migrationStatus.StatusFor(ctx, name)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return &MigrationInfo{Name: name}, nil
	}

	return &MigrationInfo{Name: name, Applied: !status.AppliedAt.IsZero()}, nil
}
