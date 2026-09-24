# `dns_test.go` Summary

Verifies the dnsRecord Norman API, testing that the schema exposes full CRUD access with correct field permissions, and that dnsRecords can be created with hostnames or IP addresses, updated, retrieved, and deleted.

## `TestDNSFields`
Retrieves the dnsRecord schema from the Norman API and verifies that it exposes full CRUD operations and expected resource fields with correct create/update permissions.
- Checks collection supports GET and POST, resource supports GET, PUT, and DELETE.
- Checks 16 specific fields including `allocateLoadBalancerNodePorts`, `clusterIPs`, `hostname`, `ipAddresses`, `ipFamilies`, `namespaceId`, `projectId`, `selector`, `targetDnsRecordIds`, `targetWorkloadIds`, and `trafficDistribution` are present with correct create/update permissions.

## `TestDNSHostname`
Creates a project and namespace, then creates a dnsRecord with a hostname field via the Norman API, updates the hostname, and verifies CRUD operations through retrieval by ID and listing.
- Checks dnsRecord is created with `baseType` "dnsRecord" and the provided hostname "target".
- Checks the hostname field can be updated to "target2" and persists through GET and LIST operations.
- Checks the record appears in the list and can be fetched by ID and deleted successfully.

## `TestDNSIPs`
Creates a dnsRecord with two IP addresses (1.1.1.1 and 2.2.2.2) via the Norman API, updates the IPs, and verifies that creating a dnsRecord with a loopback IP (127.0.0.2) in the default namespace is rejected with status 422.
- Checks dnsRecord is created with `ipAddresses` containing both provided IPs.
- Checks IP addresses can be updated to different values and persist through reload.
- Checks that loopback IP in default namespace is rejected with HTTP 422 (UnprocessableEntity).
- Checks the original record remains queryable in list after the failed loopback attempt.
