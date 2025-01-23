package migrations

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/rancher/rancher/pkg/migrations/descriptive"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
)

// UnknownMigrationError is returned for requests to operate on a migration that
// is not known to the system
type UnknownMigrationError struct {
	Name string
}

func (u UnknownMigrationError) Error() string {
	return fmt.Sprintf("unknown migration %q", u.Name)
}

// MigrationStatusClient implementations get the status of a named Migration.
type MigrationStatusClient interface {
	MigrationStatusGetter
	SetStatusFor(ctx context.Context, name string, status MigrationStatus) error
}

// Apply applies a named migration to the cluster.
//
// It generates the changes, and applies them to the cluster using the provided
// client.
//
// The status of the migration is recorded in the migrations client.
func Apply(ctx context.Context, name string, migrationStatus MigrationStatusClient, client dynamic.Interface, options descriptive.ApplyOptions, mapper meta.RESTMapper) (*descriptive.ApplyMetrics, error) {
	migration, err := migrationByName(name)
	if err != nil {
		return nil, err
	}

	// TODO
	migrationChanges, err := migration.Changes(ctx, descriptive.ClientFrom(client), MigrationOptions{})
	if err != nil {
		return nil, fmt.Errorf("calculating changes for migration %q: %w", name, err)
	}

	metrics, applyErr := descriptive.ApplyChanges(ctx, client, migrationChanges.Changes, options, mapper)
	status := MigrationStatus{
		AppliedAt: time.Now(),
		Metrics:   metrics,
	}

	if applyErr != nil {
		status.Errors = applyErr.Error()
	}

	return metrics, errors.Join(applyErr, migrationStatus.SetStatusFor(ctx, name, status))
}

// ApplyUnappliedMigrations applies all migrations that are not currently known
// to be applied.
func ApplyUnappliedMigrations(ctx context.Context, migrationStatus MigrationStatusClient, client dynamic.Interface, options descriptive.ApplyOptions, mapper meta.RESTMapper) (map[string]*descriptive.ApplyMetrics, error) {
	result := map[string]*descriptive.ApplyMetrics{}
	var err error

	for i := range knownMigrations {
		migrationName := knownMigrations[i].Name()

		info, statusErr := statusForMigration(ctx, migrationName, migrationStatus)
		if statusErr != nil {
			err = errors.Join(err, statusErr)
			// TODO: log!
			continue
		}

		if info.Applied {
			// TODO: log!
			continue
		}

		metrics, migrationErr := Apply(ctx, migrationName, migrationStatus, client, options, mapper)
		if migrationErr != nil {
			err = errors.Join(err, migrationErr)
			// TODO: log!
		}

		result[knownMigrations[i].Name()] = metrics
	}

	return result, err
}

func migrationByName(name string) (Migration, error) {
	var migration Migration
	for _, v := range knownMigrations {
		if v.Name() == name {
			migration = v
			break
		}
	}

	if migration == nil {
		return nil, UnknownMigrationError{Name: name}
	}

	return migration, nil
}

// NameForMigration returns a DNS1035 compatible name for the import path for
// this migration.
func NameForMigration(v Migration) string {
	vt := reflect.TypeOf(v)
	if vt.Kind() != reflect.Pointer {
		return vt.String()
	}

	return vt.Elem().String()
}
