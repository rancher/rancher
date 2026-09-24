# `projects_test.go` Summary

Verifies role-based access control on projects, resource quotas, namespace lifecycle, and the system project.

## `TestProjectCreatorGetsOwnerBindings`
User with cluster-member role creates a project and namespace, then verifies they have owner access via project-owner bindings.
- Checks user can create a project with cluster-member role.
- Checks user can list pods, create deployments, and list metrics in their project namespace.
- Checks user has a project-owner role binding in the namespace.

## `TestReadOnlyCannotEditSecret`
Binds a user to "read-only" role on a project and attempts secret operations.
- Checks read-only user cannot create a secret.
- Checks read-only user cannot update an admin-created secret (forbidden).

## `TestReadOnlyCannotMoveNamespace`
Creates two projects, binds a user to "read-only" on both, then attempts to move a namespace between them.
- Checks read-only user can see both project namespaces.
- Checks read-only user cannot update the projectId annotation to move a namespace (forbidden).

## `TestSystemProjectCreated`
Lists all projects in the cluster and checks for System and Default projects.
- Checks System project exists with label "authz.management.cattle.io/system-project"="true".
- Checks Default project exists with label "authz.management.cattle.io/default-project"="true".

## `TestSystemProjectCannotBeDeleted`
Retrieves the System project and attempts to delete it.
- Checks deletion returns 405 Method Not Allowed.
- Checks error message contains "System Project cannot be deleted".

## `TestSystemNamespacesDefaultServiceAccount`
Reads the system-namespaces setting and checks the default ServiceAccount in each system namespace.
- Checks each default service account in system namespaces (except kube-system) has automountServiceAccountToken=false.

## `TestProjectResourceQuotaFields`
Creates a project with resource quota and namespace default resource quota, then retrieves it.
- Checks project.resourceQuota.limit.pods="100".
- Checks project.namespaceDefaultResourceQuota.limit.pods="100".

## `TestProjectQuotaAPIValidation`
Tests various invalid quota configurations: resourceQuota without namespaceDefaultResourceQuota, vice versa, exceeding limits, and missing fields.
- Checks resourceQuota without namespaceDefaultResourceQuota fails with 422.
- Checks namespaceDefaultResourceQuota without resourceQuota fails with 422.
- Checks namespace quota exceeding project quota fails with 422.
- Checks namespace quota missing fields defined on project quota fails with 422.

## `TestProjectContainerDefaultResourceLimit`
Creates a project with containerDefaultResourceLimit (CPU/memory requests and limits), then clears it.
- Checks project stores the limits correctly.
- Checks updating with null clears the limit.

## `TestNamespaceResourceQuotaCreated`
Creates a project with quota and namespace with explicit quota annotation requesting 4 pods.
- Checks a k8s ResourceQuota is created with pods limit=4.

## `TestNamespaceDefaultQuotaApplied`
Creates a project with namespace default quota of 4 pods and a namespace without explicit quota.
- Checks the k8s ResourceQuota is created with the project's default limit of 4 pods.

## `TestProjectUsedQuotaUpdated`
Creates a project with quota and a namespace with default quota.
- Checks the project's usedLimit.pods is updated to 4 (the namespace quota).

## `TestProjectQuotaUpdateAppliedToNamespace`
Creates a project without quota, adds a namespace, then updates the project to add quota.
- Checks the controller applies the default quota to the existing namespace.

## `TestProjectUsedQuotaExactMatch`
Creates a project with 10 pod limit, then creates two namespaces using 2 and 8 pods respectively (totaling 10).
- Checks the project's usedLimit is 10.
- Checks reducing the project quota below 10 fails with 422.

## `TestProjectQuotaAddRemoveFields`
Creates a project with pod quota, adds two namespaces, then adds/removes a services field.
- Checks adding services with invalid default fails with 422.
- Checks adding services with valid default succeeds and controller propagates to existing namespaces.
- Checks removing the services field succeeds.

## `TestProjectQuotaCannotExceedWithExistingNamespaces`
Creates a project with 4 namespaces, then attempts to set quota where default × namespace count exceeds limit.
- Checks setting quota where 2 pods default × 4 namespaces = 8 > 5 limit fails with 422.

## `TestNamespaceQuotaExceedsProjectLimit`
Creates namespace requesting more pods (200) than the project allows (100).
- Checks a k8s ResourceQuota is created but with zeroed overused resources.
