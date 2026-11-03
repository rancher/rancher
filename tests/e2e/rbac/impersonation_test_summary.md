# `impersonation_test.go` Summary

Verifies that impersonation permissions are correctly granted by cluster roles and that limited impersonation via a custom ClusterRole and ClusterRoleBinding works as expected.

## `TestImpersonationByClusterRole`
**Arrange:**
- Creates user1 bound to "cluster-member" on the local cluster via CRTB, and user2 bound to "cluster-owner" via CRTB.
- Confirms baseline impersonation permissions: the admin can always impersonate; user1 (cluster-member) cannot impersonate; user2 (cluster-owner) can impersonate any user.

**Act:** Creates a ClusterRole scoping "impersonate" on "users" to user2's ID only, and binds user1 to it via a ClusterRoleBinding.

**Assert:**
- Checks user1 can now impersonate user2 specifically, due to the scoped ClusterRoleBinding.
