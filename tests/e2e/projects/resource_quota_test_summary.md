# `resource_quota_test.go` Summary

Verifies that resource quotas are correctly created, overridden, and propagated when projects and namespaces are created or updated.

## `TestCreateNamespaceWithQuotaInProject`
**Arrange:**
- Creates a project with a resource quota limit of 500m CPU and a namespace-default resource quota limit of 200m CPU.

**Act:** Creates a namespace in that project.

**Assert:**
- Checks exactly 1 resource quota exists in the namespace.
- Checks the quota's CPU limit matches the namespace default (200m).

## `TestCreateNamespaceWithOverriddenQuotaInProject`
**Arrange:**
- Creates a project with a resource quota limit of 500m CPU and a namespace-default limit of 200m CPU.

**Act:** Creates two namespaces in the project — one annotated to override its quota to 190m CPU, the other annotated to override to 400m CPU plus 50 ConfigMaps.

**Assert:**
- Checks the first namespace's quota is 190m CPU (an override below the default is honored).
- Checks the second namespace's quota has CPU reset to 0 (the 400m override exceeds the project limit, so CPU is dropped instead of applied).

## `TestRemoveQuotaFromProjectWithNamespacePropagation`
**Arrange:**
- Creates a project with resource quota limits of 500m CPU and 10 ConfigMaps, and namespace-default limits of 200m CPU and 5 ConfigMaps.
- Creates a namespace in that project.

**Act 1:** Removes the CPU limit from the project and its namespace default.
**Assert 1:**
- Checks the namespace's resource quota retains just the ConfigMaps limit (5).

**Act 2:** Removes the ConfigMaps limit as well.
**Assert 2:**
- Checks the namespace's resource quota object is deleted entirely (detected via watch).

## `TestAddQuotaFromProjectWithNamespacePropagation`
**Arrange:**
- Creates a project with a resource quota limit of 500m CPU and a namespace-default limit of 200m CPU.
- Creates a namespace in that project.

**Act:** Adds a Secrets limit to the project (20) and its namespace default (10), then updates the project.

**Assert:**
- Checks the namespace's resource quota updates to include both the existing CPU limit (200m) and the new Secrets limit (10).
