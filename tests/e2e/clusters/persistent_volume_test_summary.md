# `persistent_volume_test.go` Summary

Verifies that persistent volume source fields are protected from mutation and that the source type cannot be changed once set.

## `TestPersistentVolumeUpdate`
Creates a Cinder-backed persistent volume, then attempts to update its read-only fields and change its source type to azureFile.
- Checks that updating the cinder.readOnly field to true fails; the field remains false.
- Checks that attempting to add an azureFile source while keeping cinder does not add azureFile; the source type cannot be changed.
