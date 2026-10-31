# `etcdbackups_test.go` Summary

Verifies that the "backups-manage" ClusterRoleTemplate grants access to etcdbackups resources and that standard users cannot access them.

## `TestBackupsManageRole`
**Arrange:**
- Creates a restricted user with the "user-base" global role.

**Act:** Binds the restricted user to the "backups-manage" ClusterRoleTemplate on the local cluster via a CRTB.

**Assert:**
- Checks the user is eventually able to list "etcdbackups" resources (management.cattle.io) in the local cluster's namespace.

## `TestStandardUsersCannotAccessBackups`
**Arrange:**
- Creates a standard user with only the "user" global role.

**Act:** Repeatedly checks whether the user can list "etcdbackups" resources in the local cluster's namespace, to allow time for RBAC to sync.

**Assert:**
- Checks the user is never granted access to list etcdbackups — the standard "user" role does not grant it.
