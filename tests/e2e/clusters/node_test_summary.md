# `node_test.go` Summary

Verifies that the Norman schemas for the node type and node driver configurations expose the correct CRUD operations and field permissions, and do not leak sensitive filesystem paths.

## `TestNodeFields`
**Act:** Fetches the Norman schema for the node type.

**Assert:**
- Checks the schema's collection methods include GET and POST.
- Checks the schema's resource methods include GET, PUT, and DELETE.
- Checks 30+ explicit fields have the correct create/update permissions (e.g. `allocatable` is read-only, `labels` is create-update, `clusterId` is create-only).
- Checks all fields ending in "Config" are create-only, except `customConfig` which is create-update.

## `TestNodeDriverSchema`
**Act:** Fetches the `amazonec2config`, `digitaloceanconfig`, and `azureconfig` schemas.

**Assert:**
- Checks none of the schemas expose the `sshKeypath`, `sshKeyPath`, or `existingKeyPath` fields.

## `TestAmazonNodeDriverSchema`
**Act:** Fetches the `amazonec2config` schema.

**Assert:**
- Checks the schema contains the `encryptEbsVolume` field.
