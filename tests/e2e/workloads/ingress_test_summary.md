# `ingress_test.go` Summary

Verifies the Norman API for ingress resources, covering the exposed schemas' field permissions and that ingress rules can be created and are merged when they share the same host and path.

## `TestIngressFields`
**Arrange:**
- Creates a project.

**Act:** Retrieves the Norman schemas for the ingress, ingressBackend, ingressRule, and httpIngressPath types.

**Assert:**
- Checks the ingress schema supports full CRUD, with `namespaceId`/`projectId` create-only, `rules`/`tls`/`ingressClassName`/`backend`/`defaultBackend` create+update, and `publicEndpoints`/`status` read-only.
- Checks ingressBackend, ingressRule, and httpIngressPath each expose their fields as create+update.

## `TestIngress`
**Arrange:**
- Creates a workload.

**Act:** Creates an ingress with a single rule (host "foo.com", path "/", targetPort 80) referencing the workload.

**Assert:**
- Checks the ingress is created with one rule for host "foo.com".
- Checks the rule's path entry has path "/", targetPort 80, and the workload's ID in `workloadIds`.
- Checks `serviceId` is nil.

## `TestIngressRulesSameHostPortPath`
**Arrange:**
- Creates two workloads.

**Act:** Creates an ingress with two rules that share the same host ("foo.com"), path ("/"), and targetPort (80) but reference different workloads.

**Assert:**
- Checks the two rules are merged into a single rule with one path entry.
- Checks the merged path entry's `workloadIds` contains both workloads' IDs.
- Checks `serviceId` remains nil.
