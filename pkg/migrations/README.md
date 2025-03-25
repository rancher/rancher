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

`./preferences` is an example migration that can combine multiple `v3.Preference` resources into a single `ConfigMap` and removes the original `v3.Preference` resources.

`./restrictedadmin` is an example migration that replaces GlobalRoleBindings
that reference `restricted-admin` with `restricted-admin-replacement`.

`./sample` is another example migration that modifies existing Namespace
resources, simply adding an annotation.

`./test` is some extracted functions that were reused during writing tests for
this.

# Things to do in Migrations

* When rerunning an incomplete migration, add the new data to any existing data.
* Ensure that it's clear from the status that it's incomplete.
