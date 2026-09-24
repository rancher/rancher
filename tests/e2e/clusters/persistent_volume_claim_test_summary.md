# `persistent_volume_claim_test.go` Summary

Verifies that PVC creation validates Azure StorageClass parameters and handles various storage configuration scenarios correctly.

## `TestCannotCreateAzureNoAccountStorageType`
Creates an Azure-disk StorageClass via the k8s API (bypassing Norman defaults) without storageaccounttype or skuName parameters, then attempts to create a PVC referencing it.
- Checks the PVC creation request is rejected with HTTP 422 (Unprocessable Entity).
- Checks the error message contains "must provide storageaccounttype or skuName".

## `TestCanCreateAzureAnyAccountStorageType`
Creates Azure-disk StorageClasses with either storageaccounttype or skuName parameters, then creates PVCs referencing each.
- Checks a PVC referencing a StorageClass with storageaccounttype can be created successfully (HTTP 2xx).
- Checks a PVC referencing a StorageClass with skuName can be created successfully (HTTP 2xx).

## `TestCanCreatePVCNoStorageNoVol`
Creates a PVC without specifying a storage class or volume reference.
- Checks the PVC is created successfully (HTTP 2xx).
- Checks the PVC starts in the "pending" state.
