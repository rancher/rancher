# `etcdbackups_test.go` Summary

Verifies that the "backups-manage" ClusterRoleTemplate grants access to etcdbackups resources and that standard users cannot access them.

## `TestBackupsManageRole`
Creates a restricted user and binds them to the "backups-manage" ClusterRoleTemplate on the local cluster, then verifies access propagates.
- Checks the user can list etcdbackups in the cluster's namespace via RBAC after propagation.

## `TestStandardUsersCannotAccessBackups`
Creates a standard user with only the "user" global role and waits for RBAC to propagate.
- Checks the user cannot list etcdbackups in the cluster's namespace.
