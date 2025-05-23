Exploratory Migrations
======================

This is just a PoC of migrating "database-style" resources in Rancher.

`apply.go` contains the high-level `ApplyUnappliedMigrations` function which applies
migrations that are not recorded as having been applied.

`list.go` contains the `knownMigrations`,  migrations `Register()` themselves
with this list in code.

`client.go` contains the "status client" which is the recording mechanism for
the state of applied migrations.

`./example` contains a couple of migrations that demonstrate batching and
limiting of requests.

`./changes` contains the core mechanisms for defining a set of changes to be
applied to a cluster (which is what a `Migration` is), and applying changes to
resources.

`./preferences` is an example migration that can combine multiple
`v3.Preference` resources into a single `ConfigMap` and removes the original
`v3.Preference` resources.

Each user is modified as part of a ChangeSet which means that failures to modify
one user's preferences doesn't impact on another user's changes.

`./restrictedadmin` is an example migration that replaces GlobalRoleBindings
that reference `restricted-admin` with `restricted-admin-replacement`.

`./sample` is another example migration that modifies existing Namespace
resources, simply adding an annotation.

`./test` is some extracted functions that were reused during writing tests for
this.

## Migration model

A migration is defined by an interface:

```go
type Migration interface {
	Name() string

	// Changes should return the set of changes that this migration wants to
	// apply to the cluster.
	Changes(ctx context.Context, client changes.Interface, opts MigrationOptions) (*MigrationChanges, error)
}
```

To be applied automatically by `ApplyUnappliedMigrations` a `Migration` implementation needs to be registered.

The simplest way to do this is via:

```
func init() {
	// migrations.Register(exampleMigration{})
}
```

This will register the `exampleMigration` value.

### MigrationChanges

```go
// A migration is represented as a set of ChangeSets.
type ChangeSet []changes.ResourceChange

// MigrationChanges represents the calculated changes to apply to the cluster.
type MigrationChanges struct {
	Continue string
	Changes  []ChangeSet
}

```

This is the output from a `Changes` function, if a `Migration` is not returning all the modifications it needs to make it can return a `Continue` value which is an opaque value that the `Migration` understands that can be used to continue from the previous `Changes` call.

This is intended to follow the same mechanism as the Kubernetes [`ListMeta`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ListMeta) `Continue` value.

It's up to the `Migration` to store a value in there that it can parse, it MUST return an empty string if there are no further elements to be applied.

The other part of the `MigrationChanges` return value is a slice of `ChangeSet` values, a `ChangeSet` is a slice of `ResourceChanges`, each `ResourceChange` should normally result in a call to the Kubernetes api-server to create, delete or modify an existing resource.

```
// ResourceChange is a change to be applied in-cluster.
type ResourceChange struct {
	Operation string `json:"op"`

	Patch  *PatchChange  `json:"patch,omitempty"`
	Create *CreateChange `json:"create,omitempty"`
	Delete *DeleteChange `json:"delete,omitempty"`
}
```

Thus a `ChangeSet` is a modification of the cluster, some migrations might return a single `ChangeSet` if they only want to create/modify a single resource.

Within the scope of a `ChangeSet` a failure will result in failure to apply the `ChangeSet`, if the second `ResourceChange` fails, the remainder of the `ResourceChanges` will **not** be applied.

Multiple `ChangeSet` values might be returned if there is more than one independent set of changes, in this case, a failure to apply a `ChangeSet` will not prevent a continuation of subsequent `ChangeSets`.

Thus a `ChangeSet` is a failure boundary, failure to apply a `ResourceChange` in a `ChangeSet` will end the `ChangeSet` but not the application of the `MigrationChanges`.

# Things to do in Migrations

* When rerunning an incomplete migration, add the new data to any existing data.
* Ensure that it's clear from the status that it's incomplete.
