package migrations

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"

	"github.com/rancher/rancher/pkg/migrations/changes"
)

var logger = logrus.WithFields(logrus.Fields{"process": "migrations"})

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
func Apply(ctx context.Context, name string, migrationStatus MigrationStatusClient, client dynamic.Interface, options changes.ApplyOptions, mapper meta.RESTMapper) (*changes.ApplyMetrics, error) {
	migration, err := migrationByName(name)
	if err != nil {
		return nil, err
	}
	logger := logger.WithFields(logrus.Fields{"apply": name, "dryrun": options.DryRun})
	logger.Info("migration started")

	// TODO Loop while migrationChanges.Continue != ""
	// TODO Introduce some sort of delay - can be configured by the migration?
	migrationChanges, err := migration.Changes(ctx, changes.ClientFrom(client), MigrationOptions{})
	if err != nil {
		status := MigrationStatus{
			Errors: err.Error(),
		}
		// TODO Log the error?
		return nil, errors.Join(fmt.Errorf("calculating changes for migration %q: %w", name, err), migrationStatus.SetStatusFor(ctx, name, status))
	}

	// TODO Should this retry?
	start := time.Now()
	metrics, applyErr := changes.ApplyChanges(ctx, client, migrationChanges.Changes, options, mapper)
	status := MigrationStatus{
		AppliedAt: time.Now(),
		Metrics:   metrics,
	}
	logger.WithFields(metricsToFields(metrics)).
		WithFields(logrus.Fields{"duration": time.Since(start)}).
		Info("migration ended")

	if applyErr != nil {
		status.Errors = applyErr.Error()
	}

	return metrics, errors.Join(applyErr, migrationStatus.SetStatusFor(ctx, name, status))
}

func metricsToFields(m *changes.ApplyMetrics) logrus.Fields {
	return logrus.Fields{
		"create": m.Create,
		"delete": m.Delete,
		"patch":  m.Patch,
		"errors": m.Errors,
	}
}

// ApplyUnappliedMigrations applies all migrations that are not currently known
// to be applied.
//
// The state of the applied migrations is recorded, and metrics per-migration
// for each of the applied migrations is applied.
func ApplyUnappliedMigrations(ctx context.Context, migrationStatus MigrationStatusClient, client dynamic.Interface, options changes.ApplyOptions, mapper meta.RESTMapper) (map[string]*changes.ApplyMetrics, error) {
	result := map[string]*changes.ApplyMetrics{}
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
			logger.WithFields(logrus.Fields{"dryrun": options.DryRun, "migration": migrationName}).
				Debug("Migration skipped - already applied")
			continue
		}

		metrics, migrationErr := Apply(ctx, migrationName, migrationStatus, client, options, mapper)
		if migrationErr != nil {
			err = errors.Join(err, migrationErr)
			// TODO: log!
		}

		result[knownMigrations[i].Name()] = metrics
	}

	// TODO Should a migration be able to indicate that failure-to-apply is
	// terminal?

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

// NameForMigration returns a name for the import path for
// this migration.
func NameForMigration(v Migration) string {
	vt := reflect.TypeOf(v)
	if vt.Kind() != reflect.Pointer {
		return vt.String()
	}

	return vt.Elem().String()
}
