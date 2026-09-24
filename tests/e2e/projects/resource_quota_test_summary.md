# `resource_quota_test.go` Summary

Verifies that resource quotas are correctly created, overridden, and propagated when projects and namespaces are created or updated.

## `TestCreateNamespaceWithQuotaInProject`
Creates a project with resource quota limit of 500m CPU and namespace default of 200m CPU, then creates a namespace in that project.
- Checks exactly 1 resource quota is created in the namespace.
- Checks the quota contains the namespace default limit of 200m CPU.

## `TestCreateNamespaceWithOverriddenQuotaInProject`
Creates a project with resource quota 500m CPU and namespace default 200m CPU, then creates two namespaces with overridden quotas (190m CPU and 400m CPU + 50 ConfigMaps via annotations).
- Checks the first namespace has 190m CPU quota (lower than default).
- Checks the second namespace has 0 CPU quota (400m exceeds project limit, so CPU is removed).

## `TestRemoveQuotaFromProjectWithNamespacePropagation`
Creates a project and namespace with CPU 500m project limit, 200m namespace default, and ConfigMaps 10 project, 5 namespace; then removes CPU limits from both project and namespace defaults; then removes ConfigMaps limits.
- Checks after removing CPU limits, the namespace quota retains only ConfigMaps limit of 5.
- Checks after removing ConfigMaps limits, the namespace has no resource quotas (deletion detected via watch).

## `TestAddQuotaFromProjectWithNamespacePropagation`
Creates a project and namespace with CPU 500m project limit and 200m namespace default, then adds Secrets limit 20 to project and 10 to namespace default.
- Checks the namespace quota is updated to include both CPU 200m and Secrets 10.
