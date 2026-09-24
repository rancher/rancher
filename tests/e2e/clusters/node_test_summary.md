# `node_test.go` Summary

Verifies that the Norman schema for node types and node driver configurations expose the correct CRUD operations, field permissions, and do not leak sensitive filesystem paths.

## `TestNodeFields`
Fetches the node schema and verifies that CRUD methods are available and field-level create/update permissions match expectations.
- Checks the node schema supports GET and POST collection methods.
- Checks the node schema supports GET, PUT, and DELETE resource methods.
- Checks 30+ explicit fields have the correct create/update permissions (e.g., allocatable is read-only, labels are create-update, clusterId is create-only).
- Checks all fields ending in "Config" are create-only, except customConfig which is create-update.

## `TestNodeDriverSchema`
Fetches the amazonec2config, digitaloceanconfig, and azureconfig schemas and verifies they do not expose sensitive filesystem path fields.
- Checks that sshKeypath, sshKeyPath, and existingKeyPath fields are not present in driver schemas.

## `TestAmazonNodeDriverSchema`
Fetches the amazonec2config schema and verifies it includes fields required for EBS integration.
- Checks the amazonec2config schema contains the encryptEbsVolume field.
