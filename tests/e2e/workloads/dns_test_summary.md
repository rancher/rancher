# `dns_test.go` Summary

Verifies the dnsRecord Norman API, covering the exposed schema's CRUD permissions and the full create/update/list/delete lifecycle for dnsRecords created with a hostname or with IP addresses.

## `TestDNSFields`
**Arrange:**
- Creates a project.

**Act:** Retrieves the dnsRecord schema from the Norman API.

**Assert:**
- Checks the collection supports GET and POST, and the resource supports GET, PUT, and DELETE.
- Checks 16 fields (including `allocateLoadBalancerNodePorts`, `clusterIPs`, `hostname`, `ipAddresses`, `ipFamilies`, `namespaceId`, `projectId`, `selector`, `targetDnsRecordIds`, `targetWorkloadIds`, `trafficDistribution`) are present with the expected create/update permissions.

## `TestDNSHostname`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a dnsRecord with hostname "target" in the namespace.

**Assert:**
- Checks the record is created with `baseType`/`type` "dnsRecord" and hostname "target".
- Checks the hostname can be updated to "target2" and the change persists through GET and reload.
- Checks the record appears in the list, can be fetched by ID, and can be deleted.

## `TestDNSIPs`
**Arrange:**
- Creates a project and a namespace.

**Act:** Creates a dnsRecord with IP addresses 1.1.1.1 and 2.2.2.2 in the namespace.

**Assert:**
- Checks the record is created with `ipAddresses` containing both IPs.
- Checks the IPs can be updated to different values and the change persists through reload.
- Checks creating a dnsRecord with a loopback IP (127.0.0.2) in the default namespace is rejected with HTTP 422.
- Checks the original record still appears in the list after the rejected attempt.
