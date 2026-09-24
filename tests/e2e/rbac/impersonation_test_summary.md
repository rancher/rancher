# `impersonation_test.go` Summary

Verifies that impersonation permissions are correctly granted by cluster roles and that limited impersonation via ClusterRole and ClusterRoleBinding works as expected.

## `TestImpersonationByClusterRole`
Creates two users with cluster-member and cluster-owner roles respectively, then creates a ClusterRole limiting impersonation to one user and binds it to the other.
- Checks admin can always impersonate.
- Checks cluster-member user cannot impersonate.
- Checks cluster-owner user can impersonate any user.
- Checks limited impersonation ClusterRole restricts user1 to impersonating only user2.
