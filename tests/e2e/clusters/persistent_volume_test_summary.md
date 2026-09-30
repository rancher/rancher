# `persistent_volume_test.go` Summary

Verifies that a persistent volume's source fields are protected from mutation and that its source type cannot be changed after creation.

## `TestPersistentVolumeUpdate`
**Arrange:**
- Creates a Cinder-backed persistent volume.

**Act:** Sends PUT requests attempting to mutate the `cinder.readOnly` field and to replace the volume's source with an `azureFile` entry.

**Assert:**
- Checks `cinder.readOnly` remains false; the field was not mutated.
- Checks the `azureFile` field is absent afterward; the source type could not be changed.
