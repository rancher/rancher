# `persistent_volume_claim_test.go` Summary

Verifies that PVC creation validates Azure StorageClass parameters and correctly handles missing storage configuration.

## `TestCannotCreateAzureNoAccountStorageType`
**Arrange:**
- Creates a project and namespace.
- Creates an azure-disk StorageClass directly via the k8s API (bypassing Norman's default-filling) with no `storageaccounttype` or `skuName` parameter.

**Act:** Creates a PVC referencing that StorageClass.

**Assert:**
- Checks the request is rejected with HTTP 422 (Unprocessable Entity).
- Checks the error message contains "must provide storageaccounttype or skuName".

## `TestCanCreateAzureAnyAccountStorageType`
**Arrange:**
- Creates a project and namespace.
- Creates two azure-disk StorageClasses via the Norman API: one with a `storageaccounttype` parameter, one with a `skuName` parameter.

**Act:** Creates a PVC referencing each StorageClass.

**Assert:**
- Checks the PVC referencing the `storageaccounttype` StorageClass is created successfully (HTTP 2xx).
- Checks the PVC referencing the `skuName` StorageClass is created successfully (HTTP 2xx).

## `TestCanCreatePVCNoStorageNoVol`
**Arrange:**
- Creates a project and namespace.

**Act:** Creates a PVC with no storage class and no volume reference.

**Assert:**
- Checks the PVC is created successfully (HTTP 2xx).
- Checks the PVC starts in the "pending" state.
