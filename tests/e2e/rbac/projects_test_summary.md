# `projects_test.go` Summary

Verifies role-based access control on projects, resource quotas, namespace lifecycle, and the system project.

## `TestProjectCreatorGetsOwnerBindings`
**Arrange:**
- Creates a user.
- Grants the user "cluster-member" on the local cluster via CRTB.

**Act:** The user creates a project (retrying until RBAC permits it) and a namespace within it.

**Assert:**
- Checks the user can eventually create the namespace once RBAC propagates, and the project becomes active.
- Checks the user can list pods in the namespace.
- Checks the user has a `project-owner` (or `project-owner-aggregator`) RoleBinding in the namespace.
- Checks the user can create deployments (extensions group) in the namespace.
- Checks the user can list `pods.metrics.k8s.io` in the namespace.

## `TestReadOnlyCannotEditSecret`
**Arrange:**
- Creates a user and binds them to "read-only" on the suite's shared project (local cluster) via PRTB.
- Creates a namespace in the project.
- Admin creates a secret in the namespace (for the update check).

**Act:** The read-only user attempts to create a new secret in the namespace and attempts to update the admin-created secret.

**Assert:**
- Checks creating the secret is forbidden.
- Checks updating the existing secret is forbidden.

## `TestReadOnlyCannotMoveNamespace`
**Arrange:**
- Creates a user.
- Creates two projects (p1, p2) on the local cluster, waiting for their project namespaces to exist.
- Binds the user to "read-only" on both projects via PRTBs.
- Creates a namespace in project 1, and waits for the user to be able to see it.

**Act:** The read-only user attempts to move the namespace to project 2 by patching its `field.cattle.io/projectId` annotation.

**Assert:**
- Checks the patch attempt is forbidden.

## `TestSystemProjectCreated`
**Act:** Lists all projects in the local cluster.

**Assert:**
- Checks the Default project exists with label `authz.management.cattle.io/default-project`="true".
- Checks the System project exists with label `authz.management.cattle.io/system-project`="true".

## `TestSystemProjectCannotBeDeleted`
**Arrange:**
- Retrieves the System project from the local cluster's project list.

**Act:** Attempts to delete the System project.

**Assert:**
- Checks deletion fails with 405 Method Not Allowed.
- Checks the error message contains "System Project cannot be deleted".

## `TestSystemNamespacesDefaultServiceAccount`
**Arrange:**
- Reads the "system-namespaces" setting to get the list of system namespace names.

**Act:** Lists the default ServiceAccount object across namespaces.

**Assert:**
- Checks every default ServiceAccount in a system namespace (excluding kube-system) has `automountServiceAccountToken=false`.

## `TestProjectResourceQuotaFields`
**Act:** Creates a project on the local cluster with a ResourceQuota (pods=100) and a NamespaceDefaultResourceQuota (pods=100).

**Assert:**
- Checks `project.resourceQuota.limit.pods` is "100".
- Checks `project.namespaceDefaultResourceQuota.limit.pods` is "100".

## `TestProjectQuotaAPIValidation`
**Act:** Attempts several invalid project quota configurations: a `resourceQuota` without a `namespaceDefaultResourceQuota`, a `namespaceDefaultResourceQuota` without a `resourceQuota`, a namespace default quota (pods=200) exceeding the project quota (pods=100), and — via update on a freshly created project — a namespace default quota missing the "services" field defined on the project quota (pods=100, services=100).

**Assert:**
- Checks `resourceQuota` without `namespaceDefaultResourceQuota` fails with 422.
- Checks `namespaceDefaultResourceQuota` without `resourceQuota` fails with 422.
- Checks the namespace default quota exceeding the project quota fails with 422.
- Checks the namespace default quota missing a field defined on the project quota fails with 422.

## `TestProjectContainerDefaultResourceLimit`
**Arrange:**
- None beyond suite defaults.

**Act 1:** Creates a project on the local cluster with a ResourceQuota (pods=100), a NamespaceDefaultResourceQuota (pods=100), and a ContainerDefaultResourceLimit (requests 1 CPU / 1Gi memory, limits 2 CPU / 2Gi memory).
**Assert 1:**
- Checks the project stores the ResourceQuota.
- Checks the project stores the ContainerDefaultResourceLimit.

**Act 2:** Updates the project, setting `containerDefaultResourceLimit` to nil.
**Assert 2:**
- Checks the project's `containerDefaultResourceLimit` becomes nil.

## `TestNamespaceResourceQuotaCreated`
**Arrange:**
- Creates a project on the local cluster with a ResourceQuota (pods=100) and a NamespaceDefaultResourceQuota (pods=100).

**Act:** Creates a namespace in the project with an explicit quota annotation requesting 4 pods.

**Assert:**
- Checks a k8s ResourceQuota object is created in the namespace with a pods limit of 4.

## `TestNamespaceDefaultQuotaApplied`
**Arrange:**
- Creates a project on the local cluster with a ResourceQuota (pods=100) and a NamespaceDefaultResourceQuota (pods=4).

**Act:** Creates a namespace in the project without an explicit quota annotation.

**Assert:**
- Checks the k8s ResourceQuota created in the namespace uses the project's default limit of 4 pods.

## `TestProjectUsedQuotaUpdated`
**Arrange:**
- Creates a project on the local cluster with a ResourceQuota (pods=100) and a NamespaceDefaultResourceQuota (pods=4).

**Act:** Creates a namespace in the project without an explicit quota, so the project's default applies.

**Assert:**
- Checks the project's `usedLimit.pods` updates to 4.

## `TestProjectQuotaUpdateAppliedToNamespace`
**Arrange:**
- Creates a project on the local cluster without a quota.
- Creates a namespace in the project (no quota exists yet).

**Act:** Updates the project to add a ResourceQuota (pods=100) and a NamespaceDefaultResourceQuota (pods=4).

**Assert:**
- Checks the controller creates a k8s ResourceQuota in the existing namespace using the new default of 4 pods.

## `TestProjectUsedQuotaExactMatch`
**Arrange:**
- Creates a project on the local cluster with a ResourceQuota (pods=10) and a NamespaceDefaultResourceQuota (pods=2).
- Creates two namespaces with explicit quotas of 2 and 8 pods respectively (totaling 10, matching the full project limit).
- Confirms the project's `usedLimit.pods` reaches 10.

**Act:** Attempts to reduce the project's quota to pods=8 (with a namespace default of pods=1).

**Assert:**
- Checks the update fails with 422 Unprocessable Entity.

## `TestProjectQuotaAddRemoveFields`
**Arrange:**
- Creates a project on the local cluster with a ResourceQuota (pods=10) and a NamespaceDefaultResourceQuota (pods=2).
- Creates two namespaces using the default quota (2 pods each); confirms the project's `usedLimit.pods` reaches 4.

**Act 1:** Attempts to add a "services" field to the project quota (services=10) and namespace default (services=7) — a default that, multiplied across the 2 existing namespaces, would exceed the project limit.
**Assert 1:**
- Checks the update fails with 422 Unprocessable Entity.

**Act 2:** Updates the project with a valid "services" default (project services=10, namespace default services=2).
**Assert 2:**
- Checks the update succeeds.
- Checks the controller propagates the new default to the existing namespaces, bringing the project's `usedLimit.services` to 4.

**Act 3:** Removes the "services" field from both the project quota and the namespace default.
**Assert 3:**
- Checks the update succeeds.

## `TestProjectQuotaCannotExceedWithExistingNamespaces`
**Arrange:**
- Creates a project on the local cluster without a quota.
- Creates 4 namespaces in the project (no quotas).

**Act:** Attempts to set the project quota to pods=5 with a namespace default of pods=2 (2 × 4 = 8 > 5).

**Assert:**
- Checks the update fails with 422 Unprocessable Entity.

## `TestNamespaceQuotaExceedsProjectLimit`
**Arrange:**
- Creates a project on the local cluster with a ResourceQuota (pods=100) and a NamespaceDefaultResourceQuota (pods=100).

**Act:** Creates a namespace in the project requesting an explicit quota of 200 pods, exceeding the project's limit.

**Assert:**
- Checks a k8s ResourceQuota is still created in the namespace.
- Checks the pods value is not set to the requested 200 (overused resources are zeroed rather than granted).
