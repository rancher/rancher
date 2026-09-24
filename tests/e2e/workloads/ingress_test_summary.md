# `ingress_test.go` Summary

Verifies the Norman API for ingress resources, testing that the ingress schema exposes correct field permissions for ingress, ingressBackend, ingressRule, and httpIngressPath types, and that ingress rules can be created, merged based on host/path matching, and properly stored.

## `TestIngressFields`
Retrieves schemas for ingress, ingressBackend, ingressRule, and httpIngressPath types from the Norman API and verifies each type exposes the expected fields with correct create/update permissions.
- Checks ingress schema supports CRUD, with fields `namespaceId` and `projectId` as create-only, fields `rules`, `tls`, `ingressClassName`, `backend`, `defaultBackend` as create+update, and `publicEndpoints` and `status` as read-only.
- Checks ingressBackend, ingressRule, and httpIngressPath expose all expected fields as create+update.

## `TestIngress`
Creates a workload, then creates an ingress with a single rule targeting that workload with host "foo.com" and path "/" on targetPort 80.
- Checks ingress is created with one rule containing the host "foo.com".
- Checks the rule's path entry has path "/", targetPort 80, and the correct workloadIds reference.
- Checks serviceId is nil for the workload-based rule.

## `TestIngressRulesSameHostPortPath`
Creates two workloads, then creates an ingress with two rules sharing the same host ("foo.com"), path ("/"), and targetPort (80) but referencing different workloads.
- Checks the two rules are merged into a single rule with one path entry.
- Checks the merged path entry contains workloadIds for both workloads.
- Checks serviceId remains nil.
